package daemon

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"cc-tui/model"
)

var nonAlphanumHyphen = regexp.MustCompile(`[^a-zA-Z0-9-]`)

func EncodeProjectPath(path string) string {
	return nonAlphanumHyphen.ReplaceAllString(path, "-")
}

func FindSessionUUID(projectPath, projectsDir string) (uuid string, jsonlPath string) {
	encoded := EncodeProjectPath(projectPath)
	projDir := filepath.Join(projectsDir, encoded)

	entries, err := os.ReadDir(projDir)
	if err != nil {
		return "", ""
	}

	type fileInfo struct {
		name  string
		mtime int64
	}
	var jsonls []fileInfo
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".jsonl" {
			info, err := e.Info()
			if err != nil {
				continue
			}
			jsonls = append(jsonls, fileInfo{e.Name(), info.ModTime().UnixMilli()})
		}
	}
	if len(jsonls) == 0 {
		return "", ""
	}

	sort.Slice(jsonls, func(i, j int) bool {
		return jsonls[i].mtime > jsonls[j].mtime
	})

	name := jsonls[0].name
	uuid = name[:len(name)-len(".jsonl")]
	return uuid, filepath.Join(projDir, name)
}

type HistoryEntry struct {
	ID          string
	Project     string
	LastTS      int64
	LastDisplay string // most recent prompt text (fallback label when transcript is gone)
}

func LoadHistory(historyPath string) ([]HistoryEntry, error) {
	f, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	type sessionAcc struct {
		id          string
		project     string
		lastTS      int64
		lastDisplay string
	}
	byID := make(map[string]*sessionAcc)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var entry struct {
			SessionID string `json:"sessionId"`
			Project   string `json:"project"`
			Timestamp int64  `json:"timestamp"`
			Display   string `json:"display"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.SessionID == "" {
			continue
		}
		acc, ok := byID[entry.SessionID]
		if !ok {
			acc = &sessionAcc{id: entry.SessionID, project: entry.Project}
			byID[entry.SessionID] = acc
		}
		// Keep the display text from the most recent entry seen.
		if entry.Timestamp >= acc.lastTS {
			acc.lastTS = entry.Timestamp
			if entry.Display != "" {
				acc.lastDisplay = entry.Display
			}
		}
		if entry.Project != "" {
			acc.project = entry.Project
		}
	}

	result := make([]HistoryEntry, 0, len(byID))
	for _, acc := range byID {
		result = append(result, HistoryEntry{
			ID:          acc.id,
			Project:     acc.project,
			LastTS:      acc.lastTS,
			LastDisplay: acc.lastDisplay,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTS > result[j].LastTS
	})
	return result, nil
}

type SessionMeta struct {
	Slug          string
	Title         string
	LastUserMsg   string
	GitBranch     string
	ContextTokens int // live context size from the last assistant message's usage
}

// sumContextTokens totals the token fields that make up the live context
// window: prompt input, cache creation, cache read, and output.
func sumContextTokens(usage map[string]interface{}) int {
	total := 0
	for _, k := range []string{"input_tokens", "cache_creation_input_tokens", "cache_read_input_tokens", "output_tokens"} {
		if v, ok := usage[k].(float64); ok {
			total += int(v)
		}
	}
	return total
}

func LoadSessionMeta(jsonlPath string) SessionMeta {
	meta := SessionMeta{}
	if jsonlPath == "" {
		return meta
	}

	f, err := os.Open(jsonlPath)
	if err != nil {
		return meta
	}
	defer f.Close()

	// Read all lines into memory so we can scan forward and backward
	var lines [][]byte
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return meta
	}

	// Forward pass: first 20 lines for slug and custom-title
	limit := 20
	if limit > len(lines) {
		limit = len(lines)
	}
	for _, line := range lines[:limit] {
		var d map[string]interface{}
		if err := json.Unmarshal(line, &d); err != nil {
			continue
		}
		if slug, ok := d["slug"].(string); ok && slug != "" && meta.Slug == "" {
			meta.Slug = slug
		}
		if dtype, ok := d["type"].(string); ok && dtype == "custom-title" && meta.Title == "" {
			if title, ok := d["customTitle"].(string); ok {
				meta.Title = title
			}
		}
		if meta.Slug != "" && meta.Title != "" {
			break
		}
	}

	// Reverse pass: last user message, git branch, slug (if still missing)
	for i := len(lines) - 1; i >= 0; i-- {
		var d map[string]interface{}
		if err := json.Unmarshal(lines[i], &d); err != nil {
			continue
		}
		dtype, _ := d["type"].(string)

		if dtype == "custom-title" && meta.Title == "" {
			if title, ok := d["customTitle"].(string); ok {
				meta.Title = title
			}
		}

		// First assistant message seen going backward is the last in the file —
		// its usage reflects the current live context size.
		if dtype == "assistant" && meta.ContextTokens == 0 {
			if msg, ok := d["message"].(map[string]interface{}); ok {
				if usage, ok := msg["usage"].(map[string]interface{}); ok {
					meta.ContextTokens = sumContextTokens(usage)
				}
			}
		}

		if dtype == "user" && meta.LastUserMsg == "" {
			msg, _ := d["message"].(map[string]interface{})
			if msg != nil {
				content, _ := msg["content"].(string)
				if content != "" && content != "exit" && content != "/exit" && content != "/compact" {
					if len(content) > 200 {
						content = content[:200]
					}
					// Clean NOW so we skip junk messages (XML-wrapped compact preambles etc.)
					cleaned := CleanMessage(content)
					if cleaned != "" {
						meta.LastUserMsg = cleaned
						if branch, ok := d["gitBranch"].(string); ok && branch != "" {
							meta.GitBranch = branch
						}
					}
				}
			}
			if slug, ok := d["slug"].(string); ok && slug != "" && meta.Slug == "" {
				meta.Slug = slug
			}
		}

		if meta.Title != "" && meta.LastUserMsg != "" && meta.Slug != "" && meta.ContextTokens != 0 {
			break
		}
	}

	return meta
}

func LoadTasks(sessionUUID, tasksDir string) []model.Task {
	dir := filepath.Join(tasksDir, sessionUUID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var tasks []model.Task
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var t model.Task
		if err := json.Unmarshal(data, &t); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks
}

func LoadTodos(sessionUUID, todosDir string) []model.Todo {
	if sessionUUID == "" {
		return nil
	}
	pattern := filepath.Join(todosDir, sessionUUID+"-agent-*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	var allTodos []model.Todo
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var items []model.Todo
		if err := json.Unmarshal(data, &items); err != nil {
			continue
		}
		allTodos = append(allTodos, items...)
	}
	return allTodos
}

var (
	stepHeadingRe    = regexp.MustCompile(`^##\s+Step\s+(\d+):\s*(.+)`)
	tableRowRe       = regexp.MustCompile(`^\|\s*(\d+)\s*\|(.+)`)
	numberedListRe   = regexp.MustCompile(`^(\d+)\.\s+(.+)`)
	numberedHeadRe   = regexp.MustCompile(`^###\s+(\d+)\.\s+(.+)`)
	boldRe           = regexp.MustCompile(`\*\*(.+?)\*\*`)
	codeRe           = regexp.MustCompile("`(.+?)`")
	strconv_atoi_err = regexp.MustCompile(`^(\d+)`)
)

func cleanStepText(text string) string {
	text = boldRe.ReplaceAllString(text, "$1")
	text = codeRe.ReplaceAllString(text, "$1")
	return strings.TrimSpace(text)
}

func LoadPlan(planPath string) *model.Plan {
	if planPath == "" {
		return nil
	}
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	var title string
	var steps []model.PlanStep

	// Pass 1: look for ## Step N: headings
	for _, line := range lines {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "# ") && !strings.HasPrefix(stripped, "## ") && title == "" {
			title = strings.TrimSpace(stripped[2:])
		}
		m := stepHeadingRe.FindStringSubmatch(stripped)
		if m != nil {
			num := 0
			if n := strconv_atoi_err.FindString(m[1]); n != "" {
				for _, c := range n {
					num = num*10 + int(c-'0')
				}
			}
			steps = append(steps, model.PlanStep{
				Num:  num,
				Text: cleanStepText(m[2]),
			})
		}
	}

	// Pass 2: table rows and numbered lists under step-related headings
	if len(steps) == 0 {
		stepHeadings := []string{
			"implementation order", "implementation steps", "build sequence",
			"execution order", "steps", "files to create/modify", "files to modify",
			"file:", "verification",
		}
		inSteps := false
		for _, line := range lines {
			stripped := strings.TrimSpace(line)
			if strings.HasPrefix(stripped, "## ") {
				heading := strings.TrimSpace(stripped[3:])
				headingLower := strings.ToLower(heading)
				inSteps = false
				for _, h := range stepHeadings {
					if strings.HasPrefix(headingLower, h) {
						inSteps = true
						break
					}
				}
				continue
			}
			if !inSteps {
				continue
			}

			// Table row: | N | desc |
			m := tableRowRe.FindStringSubmatch(stripped)
			if m != nil {
				cols := strings.Split(m[2], "|")
				var cleaned []string
				for _, c := range cols {
					c = strings.TrimSpace(c)
					c = strings.Trim(c, "*")
					if c != "" && c != "-" && !strings.HasPrefix(c, "---") {
						cleaned = append(cleaned, c)
					}
				}
				desc := ""
				maxLen := 0
				for _, c := range cleaned {
					if len(c) > maxLen {
						maxLen = len(c)
						desc = c
					}
				}
				if desc != "" {
					num := 0
					for _, c := range m[1] {
						num = num*10 + int(c-'0')
					}
					steps = append(steps, model.PlanStep{
						Num:  num,
						Text: cleanStepText(desc),
					})
				}
				continue
			}

			// Numbered list: N. text
			m = numberedListRe.FindStringSubmatch(stripped)
			if m != nil {
				num := 0
				for _, c := range m[1] {
					num = num*10 + int(c-'0')
				}
				steps = append(steps, model.PlanStep{
					Num:  num,
					Text: cleanStepText(m[2]),
				})
				continue
			}

			// ### N. text
			m = numberedHeadRe.FindStringSubmatch(stripped)
			if m != nil {
				num := 0
				for _, c := range m[1] {
					num = num*10 + int(c-'0')
				}
				steps = append(steps, model.PlanStep{
					Num:  num,
					Text: cleanStepText(m[2]),
				})
			}
		}
	}

	// Pass 3: fallback to actionable ## headings
	if len(steps) == 0 {
		skip := map[string]bool{
			"context": true, "approach": true, "key insight": true,
			"key differences": true, "rendering mode": true, "verification": true,
			"out of scope": true, "notes": true, "dependencies": true,
			"environment variables": true, "success criteria": true,
		}
		num := 0
		for _, line := range lines {
			stripped := strings.TrimSpace(line)
			if strings.HasPrefix(stripped, "## ") {
				heading := strings.TrimSpace(stripped[3:])
				headingLower := strings.ToLower(strings.TrimRight(heading, ":"))
				if skip[headingLower] || strings.HasPrefix(headingLower, "key ") {
					continue
				}
				num++
				steps = append(steps, model.PlanStep{
					Num:  num,
					Text: cleanStepText(heading),
				})
			}
		}
	}

	if title == "" && len(steps) == 0 {
		return nil
	}

	return &model.Plan{
		Title: title,
		Steps: steps,
	}
}

var wordRe = regexp.MustCompile(`\w{4,}`)

func MatchStepCompletion(steps []model.PlanStep, tasks []model.Task, todos []model.Todo) {
	type labeled struct {
		words  map[string]bool
		status string
	}
	var labels []labeled
	for _, t := range tasks {
		text := strings.ToLower(t.Subject)
		words := make(map[string]bool)
		for _, w := range wordRe.FindAllString(text, -1) {
			words[w] = true
		}
		labels = append(labels, labeled{words, t.Status})
	}
	for _, t := range todos {
		text := strings.ToLower(t.Content)
		words := make(map[string]bool)
		for _, w := range wordRe.FindAllString(text, -1) {
			words[w] = true
		}
		labels = append(labels, labeled{words, t.Status})
	}

	for i := range steps {
		stepWords := make(map[string]bool)
		for _, w := range wordRe.FindAllString(strings.ToLower(steps[i].Text), -1) {
			stepWords[w] = true
		}
		minMatch := 2
		if len(stepWords) < 2 {
			minMatch = len(stepWords)
		}

		for _, l := range labels {
			overlap := 0
			for w := range stepWords {
				if l.words[w] {
					overlap++
				}
			}
			if overlap >= minMatch {
				if l.status == "completed" {
					steps[i].Status = model.StepDone
					break
				} else if l.status == "in_progress" {
					steps[i].Status = model.StepWIP
				}
			}
		}
	}
}

var (
	ansiRe    = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	xmlTagRe  = regexp.MustCompile(`<[^>]*>?`)
	toolIDRe  = regexp.MustCompile(`toolu_\w+`)
	hexHashRe = regexp.MustCompile(`\b[a-f0-9]{7,}\b`)
	spaceRe   = regexp.MustCompile(`\s+`)
)

// Unhelpful message prefixes to skip entirely
var skipPrefixes = []string{
	"This session is being continued from a previous conversation",
	"Failed to fork",
	"/compact",
}

func CleanMessage(msg string) string {
	msg = ansiRe.ReplaceAllString(msg, "")
	msg = xmlTagRe.ReplaceAllString(msg, "")
	msg = toolIDRe.ReplaceAllString(msg, "")
	msg = hexHashRe.ReplaceAllString(msg, "")
	msg = spaceRe.ReplaceAllString(strings.TrimSpace(msg), " ")
	if len(msg) <= 5 {
		return ""
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(msg, prefix) {
			return ""
		}
	}
	return msg
}

// CleanTitle strips fork suffixes and other noise from custom titles.
func CleanTitle(title string) string {
	if title == "" {
		return ""
	}
	// Strip "(Fork)", "(Fork 2)", etc.
	title = regexp.MustCompile(`\s*\(Fork\s*\d*\)\s*$`).ReplaceAllString(title, "")
	// Skip titles that are just auto-generated noise
	if strings.HasPrefix(title, "This session") {
		return ""
	}
	if strings.HasPrefix(title, "Failed to") {
		return ""
	}
	return strings.TrimSpace(title)
}

func dirName(path string) string {
	home, _ := os.UserHomeDir()
	if path == home || path == "~" {
		return "~"
	}
	return filepath.Base(path)
}

func timeFromMillis(ms int64) time.Time {
	return time.UnixMilli(ms)
}

type Dirs struct {
	Projects string // ~/.claude/projects
	Tasks    string // ~/.claude/tasks
	Todos    string // ~/.claude/todos
	Plans    string // ~/.claude/plans
	History  string // ~/.claude/history.jsonl
	Archive  string // ~/.claude/cc-tui-archive
}

func DefaultDirs() Dirs {
	home, _ := os.UserHomeDir()
	claude := filepath.Join(home, ".claude")
	return Dirs{
		Projects: filepath.Join(claude, "projects"),
		Tasks:    filepath.Join(claude, "tasks"),
		Todos:    filepath.Join(claude, "todos"),
		Plans:    filepath.Join(claude, "plans"),
		History:  filepath.Join(claude, "history.jsonl"),
		Archive:  filepath.Join(claude, "cc-tui-archive"),
	}
}

func LoadFullSession(entry HistoryEntry, dirs Dirs) model.Session {
	s := model.Session{
		ID:         entry.ID,
		Project:    entry.Project,
		DirName:    dirName(entry.Project),
		LastActive: time.UnixMilli(entry.LastTS),
	}

	uuid, jsonlPath := FindSessionUUID(entry.Project, dirs.Projects)
	if jsonlPath != "" {
		meta := LoadSessionMeta(jsonlPath)
		s.Slug = meta.Slug
		s.Title = CleanTitle(meta.Title)
		s.GitBranch = meta.GitBranch
		s.LastMsg = CleanMessage(meta.LastUserMsg)
		s.ContextTokens = meta.ContextTokens
	}

	if uuid != "" {
		s.Tasks = LoadTasks(uuid, dirs.Tasks)
		s.Todos = LoadTodos(uuid, dirs.Todos)
	}
	if s.Slug != "" {
		s.Plan = LoadPlan(filepath.Join(dirs.Plans, s.Slug+".md"))
		if s.Plan != nil {
			MatchStepCompletion(s.Plan.Steps, s.Tasks, s.Todos)
		}
	}
	return s
}
