package daemon

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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
	ID      string
	Project string
	LastTS  int64
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
		id      string
		project string
		lastTS  int64
	}
	byID := make(map[string]*sessionAcc)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var entry struct {
			SessionID string `json:"sessionId"`
			Project   string `json:"project"`
			Timestamp int64  `json:"timestamp"`
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
		if entry.Timestamp > acc.lastTS {
			acc.lastTS = entry.Timestamp
		}
		if entry.Project != "" {
			acc.project = entry.Project
		}
	}

	result := make([]HistoryEntry, 0, len(byID))
	for _, acc := range byID {
		result = append(result, HistoryEntry{
			ID:      acc.id,
			Project: acc.project,
			LastTS:  acc.lastTS,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTS > result[j].LastTS
	})
	return result, nil
}

type SessionMeta struct {
	Slug        string
	Title       string
	LastUserMsg string
	GitBranch   string
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

		if dtype == "user" && meta.LastUserMsg == "" {
			msg, _ := d["message"].(map[string]interface{})
			if msg != nil {
				content, _ := msg["content"].(string)
				if content != "" && content != "exit" && content != "/exit" && content != "/compact" {
					if len(content) > 120 {
						content = content[:120]
					}
					meta.LastUserMsg = content
					if branch, ok := d["gitBranch"].(string); ok && branch != "" {
						meta.GitBranch = branch
					}
				}
			}
			if slug, ok := d["slug"].(string); ok && slug != "" && meta.Slug == "" {
				meta.Slug = slug
			}
		}

		if meta.Title != "" && meta.LastUserMsg != "" && meta.Slug != "" {
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
