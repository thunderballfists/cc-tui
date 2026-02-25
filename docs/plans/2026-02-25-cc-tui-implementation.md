# CC-TUI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a combined session manager + dashboard TUI backed by an always-running daemon, replacing the existing fzf/Python tooling.

**Architecture:** Single Go binary with two modes — `cc-tui serve` (daemon with fsnotify + unix socket) and `cc-tui` (Bubble Tea client). Daemon watches `~/.claude/` for session data, client renders an interactive tree view.

**Tech Stack:** Go 1.23, Bubble Tea, Lip Gloss, Bubbles (help/key/viewport), fsnotify

---

## Phase 1: Project Foundation

### Task 1: Initialize Go project

**Files:**
- Create: `~/.config/tmux/cc-tui/go.mod`
- Create: `~/.config/tmux/cc-tui/main.go`

**Step 1: Initialize module and install dependencies**

```bash
cd ~/.config/tmux/cc-tui
go mod init cc-tui
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/fsnotify/fsnotify@latest
```

**Step 2: Create main.go with subcommand dispatch**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		fmt.Println("daemon mode (not yet implemented)")
		os.Exit(0)
	}
	fmt.Println("client mode (not yet implemented)")
}
```

**Step 3: Verify it builds**

Run: `cd ~/.config/tmux/cc-tui && go build -o cc-tui . && ./cc-tui`
Expected: prints "client mode (not yet implemented)"

Run: `./cc-tui serve`
Expected: prints "daemon mode (not yet implemented)"

**Step 4: Initialize git and commit**

```bash
cd ~/.config/tmux/cc-tui
git init
echo "cc-tui" > .gitignore
git add .
git commit -m "feat: initialize cc-tui Go project"
```

---

### Task 2: Data model types

**Files:**
- Create: `model/session.go`
- Create: `model/plan.go`
- Create: `model/task.go`
- Create: `model/todo.go`

**Step 1: Create model/session.go**

```go
package model

import "time"

type Session struct {
	ID         string
	Project    string    // full path: /Users/allan.beihl/traffic-api
	DirName    string    // short name: traffic-api
	Slug       string    // CC slug from JSONL
	Title      string    // custom title if set
	LastActive time.Time
	GitBranch  string
	LastMsg    string    // last user message (cleaned)

	Plan  *Plan
	Tasks []Task
	Todos []Todo

	Active    bool
	PaneID    string // tmux pane ID if active
	PaneLabel string // tmux session:window.pane if active
}
```

**Step 2: Create model/plan.go**

```go
package model

type StepStatus int

const (
	StepPending StepStatus = iota
	StepWIP
	StepDone
)

type PlanStep struct {
	Num    int
	Text   string
	Status StepStatus
}

type Plan struct {
	Title string
	Steps []PlanStep
}
```

**Step 3: Create model/task.go**

```go
package model

type Task struct {
	ID          string `json:"id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Status      string `json:"status"` // pending, in_progress, completed
	ActiveForm  string `json:"activeForm"`
}
```

**Step 4: Create model/todo.go**

```go
package model

type Todo struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // pending, in_progress, completed
	ActiveForm string `json:"activeForm"`
}
```

**Step 5: Verify it compiles**

Run: `cd ~/.config/tmux/cc-tui && go build ./...`
Expected: no errors

**Step 6: Commit**

```bash
git add model/
git commit -m "feat: add data model types"
```

---

### Task 3: Protocol / IPC message types

**Files:**
- Create: `protocol/messages.go`

**Step 1: Create protocol/messages.go**

```go
package protocol

import "cc-tui/model"

// Client → Daemon requests
type Request struct {
	Cmd       string `json:"cmd"`                 // tree, subscribe, action
	Action    string `json:"action,omitempty"`     // open, window, new
	SessionID string `json:"session_id,omitempty"`
}

// Daemon → Client responses
type Response struct {
	Type     string          `json:"type"`     // snapshot, update, error
	Sessions []model.Session `json:"sessions,omitempty"`
	Error    string          `json:"error,omitempty"`
}
```

**Step 2: Verify and commit**

Run: `go build ./...`

```bash
git add protocol/
git commit -m "feat: add IPC protocol types"
```

---

## Phase 2: Daemon — Parsers

### Task 4: Path encoding and session UUID lookup

**Files:**
- Create: `daemon/parser.go`
- Create: `daemon/parser_test.go`

**Step 1: Write failing tests**

```go
package daemon

import "testing"

func TestEncodeProjectPath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"/Users/allan.beihl", "-Users-allan-beihl"},
		{"/Users/allan.beihl/traffic_api", "-Users-allan-beihl-traffic-api"},
		{"/Users/allan.beihl/.config/tmux", "-Users-allan-beihl--config-tmux"},
	}
	for _, tt := range tests {
		got := EncodeProjectPath(tt.input)
		if got != tt.want {
			t.Errorf("EncodeProjectPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd ~/.config/tmux/cc-tui && go test ./daemon/ -run TestEncodeProjectPath -v`
Expected: FAIL (function not defined)

**Step 3: Implement EncodeProjectPath and FindSessionUUID**

```go
package daemon

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
```

**Step 4: Run tests**

Run: `go test ./daemon/ -run TestEncodeProjectPath -v`
Expected: PASS

**Step 5: Commit**

```bash
git add daemon/
git commit -m "feat: path encoding and session UUID lookup"
```

---

### Task 5: History parser

**Files:**
- Modify: `daemon/parser.go`
- Modify: `daemon/parser_test.go`
- Create: `daemon/testdata/history.jsonl` (test fixture)

**Step 1: Create test fixture**

Create `daemon/testdata/history.jsonl` with sample JSONL entries:
```json
{"sessionId":"abc-123","project":"/Users/test/proj-a","timestamp":1700000000000}
{"sessionId":"abc-123","project":"/Users/test/proj-a","timestamp":1700001000000}
{"sessionId":"def-456","project":"/Users/test/proj-b","timestamp":1700002000000}
```

**Step 2: Write failing test**

```go
func TestLoadHistory(t *testing.T) {
	sessions, err := LoadHistory("testdata/history.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	// Should be sorted by last_ts desc
	if sessions[0].ID != "def-456" {
		t.Errorf("first session = %q, want def-456", sessions[0].ID)
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./daemon/ -run TestLoadHistory -v`
Expected: FAIL

**Step 4: Implement LoadHistory**

```go
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

type HistoryEntry struct {
	ID      string
	Project string
	LastTS  int64
}
```

Add required imports: `"bufio"`, `"encoding/json"`, `"sort"`.

**Step 5: Run tests**

Run: `go test ./daemon/ -run TestLoadHistory -v`
Expected: PASS

**Step 6: Commit**

```bash
git add daemon/
git commit -m "feat: history.jsonl parser"
```

---

### Task 6: Session metadata parser

**Files:**
- Modify: `daemon/parser.go`
- Modify: `daemon/parser_test.go`
- Create: `daemon/testdata/session.jsonl` (test fixture)

**Step 1: Create test fixture** with slug, custom-title, and user messages.

**Step 2: Write failing test** for `LoadSessionMeta(jsonlPath)` returning slug, title, lastUserMsg, gitBranch.

**Step 3: Implement LoadSessionMeta**

Logic ported from Python's `load_session_meta()`:
- Read first 20 lines for slug and custom-title
- Read last lines (reversed) for last user message, git branch
- Stop early once all fields found

**Step 4: Run tests, verify pass**

**Step 5: Commit**

---

### Task 7: Task and Todo parsers

**Files:**
- Modify: `daemon/parser.go`
- Modify: `daemon/parser_test.go`

**Step 1: Write failing tests** for `LoadTasks(sessionUUID, tasksDir)` and `LoadTodos(sessionUUID, todosDir)`.

**Step 2: Implement LoadTasks**

```go
func LoadTasks(sessionUUID, tasksDir string) []model.Task {
	dir := filepath.Join(tasksDir, sessionUUID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var tasks []model.Task
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" || e.Name()[0] == '.' {
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
```

**Step 3: Implement LoadTodos**

Same pattern — reads `{todosDir}/{uuid}-agent-*.json`, unmarshals JSON arrays of Todo items.

**Step 4: Run tests, verify pass**

**Step 5: Commit**

---

### Task 8: Plan parser

This is the most complex parser — extracts title and steps from markdown.

**Files:**
- Modify: `daemon/parser.go`
- Modify: `daemon/parser_test.go`
- Create: `daemon/testdata/sample-plan.md` (test fixture with multiple step formats)

**Step 1: Create test fixture** with all 4 step extraction patterns:
- `## Step N: Title` headings
- Table rows `| N | desc |`
- Numbered lists `N. text`
- Fallback `##` headings

**Step 2: Write failing tests**

```go
func TestLoadPlan(t *testing.T) {
	plan := LoadPlan("testdata/sample-plan.md")
	if plan == nil {
		t.Fatal("plan is nil")
	}
	if plan.Title == "" {
		t.Error("missing title")
	}
	if len(plan.Steps) == 0 {
		t.Error("no steps extracted")
	}
}
```

**Step 3: Implement LoadPlan**

Port the 3-pass logic from Python's `load_plan()`:

1. Scan for `## Step N:` headings
2. If no steps: scan for table rows and numbered lists under step-related `##` sections
3. If still no steps: fallback to actionable `##` headings (skip "context", "approach", etc.)

Strip `**bold**` and `` `code` `` from step text.

**Step 4: Run tests**

**Step 5: Also test against real plan files**

Run parser against actual files in `~/.claude/plans/` to verify:
```go
func TestLoadPlanReal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real file test")
	}
	// Test against actual plan files
	plans, _ := filepath.Glob(os.ExpandEnv("$HOME/.claude/plans/*.md"))
	for _, p := range plans {
		plan := LoadPlan(p)
		t.Logf("%s: title=%q steps=%d", filepath.Base(p), plan.Title, len(plan.Steps))
	}
}
```

Run: `go test ./daemon/ -run TestLoadPlanReal -v`

**Step 6: Commit**

---

### Task 9: Step completion matching

**Files:**
- Modify: `daemon/parser.go`
- Modify: `daemon/parser_test.go`

**Step 1: Write failing test**

```go
func TestMatchStepCompletion(t *testing.T) {
	steps := []model.PlanStep{
		{Text: "Set up project structure"},
		{Text: "Implement REST endpoints"},
		{Text: "Write integration tests"},
	}
	tasks := []model.Task{
		{Subject: "Set up the project structure and config", Status: "completed"},
		{Subject: "Implement REST endpoints for users", Status: "in_progress"},
	}
	MatchStepCompletion(steps, tasks, nil)
	if steps[0].Status != model.StepDone {
		t.Errorf("step 0 = %v, want Done", steps[0].Status)
	}
	if steps[1].Status != model.StepWIP {
		t.Errorf("step 1 = %v, want WIP", steps[1].Status)
	}
	if steps[2].Status != model.StepPending {
		t.Errorf("step 2 = %v, want Pending", steps[2].Status)
	}
}
```

**Step 2: Implement MatchStepCompletion**

Port from Python: extract 4+ char words, require 2+ matching words overlap.

```go
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
```

**Step 3: Run tests**

Run: `go test ./daemon/ -run TestMatchStepCompletion -v`
Expected: PASS

**Step 4: Commit**

---

### Task 10: Message sanitizer and full session loader

**Files:**
- Modify: `daemon/parser.go`
- Modify: `daemon/parser_test.go`

**Step 1: Implement CleanMessage**

```go
var (
	xmlTagRe  = regexp.MustCompile(`<[^>]*>?`)
	toolIDRe  = regexp.MustCompile(`toolu_\w+`)
	hexHashRe = regexp.MustCompile(`\b[a-f0-9]{7,}\b`)
	spaceRe   = regexp.MustCompile(`\s+`)
)

func CleanMessage(msg string) string {
	msg = xmlTagRe.ReplaceAllString(msg, "")
	msg = toolIDRe.ReplaceAllString(msg, "")
	msg = hexHashRe.ReplaceAllString(msg, "")
	msg = spaceRe.ReplaceAllString(strings.TrimSpace(msg), " ")
	if len(msg) <= 5 {
		return ""
	}
	return msg
}
```

**Step 2: Implement LoadFullSession** — combines all parsers into a complete `model.Session`:

```go
func LoadFullSession(entry HistoryEntry, dirs Dirs) model.Session {
	s := model.Session{
		ID:      entry.ID,
		Project: entry.Project,
		DirName: dirName(entry.Project),
		LastActive: time.UnixMilli(entry.LastTS),
	}

	uuid, jsonlPath := FindSessionUUID(entry.Project, dirs.Projects)
	if jsonlPath != "" {
		meta := LoadSessionMeta(jsonlPath)
		s.Slug = meta.Slug
		s.Title = meta.Title
		s.GitBranch = meta.GitBranch
		s.LastMsg = CleanMessage(meta.LastUserMsg)
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

type Dirs struct {
	Projects string // ~/.claude/projects
	Tasks    string // ~/.claude/tasks
	Todos    string // ~/.claude/todos
	Plans    string // ~/.claude/plans
	History  string // ~/.claude/history.jsonl
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
	}
}
```

**Step 3: Test with real data**

```go
func TestLoadFullSessionReal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dirs := DefaultDirs()
	entries, _ := LoadHistory(dirs.History)
	for _, e := range entries[:min(3, len(entries))] {
		s := LoadFullSession(e, dirs)
		t.Logf("%s: title=%q plan=%v tasks=%d todos=%d",
			s.DirName, s.Title, s.Plan != nil, len(s.Tasks), len(s.Todos))
	}
}
```

**Step 4: Commit**

---

## Phase 3: Daemon — Infrastructure

### Task 11: In-memory cache

**Files:**
- Create: `daemon/cache.go`
- Create: `daemon/cache_test.go`

**Step 1: Implement Cache** — thread-safe session store with load/refresh.

```go
package daemon

import (
	"cc-tui/model"
	"sort"
	"sync"
)

type Cache struct {
	mu       sync.RWMutex
	sessions []model.Session
	dirs     Dirs
}

func NewCache(dirs Dirs) *Cache {
	return &Cache{dirs: dirs}
}

func (c *Cache) Reload() error {
	entries, err := LoadHistory(c.dirs.History)
	if err != nil {
		return err
	}

	sessions := make([]model.Session, 0, len(entries))
	for _, e := range entries {
		s := LoadFullSession(e, c.dirs)
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Active != sessions[j].Active {
			return sessions[i].Active
		}
		return sessions[i].LastActive.After(sessions[j].LastActive)
	})

	if len(sessions) > 25 {
		sessions = sessions[:25]
	}

	c.mu.Lock()
	c.sessions = sessions
	c.mu.Unlock()
	return nil
}

func (c *Cache) Sessions() []model.Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]model.Session, len(c.sessions))
	copy(result, c.sessions)
	return result
}

func (c *Cache) UpdateActiveStatus(activePanes map[string]PaneInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.sessions {
		info, ok := activePanes[c.sessions[i].Project]
		c.sessions[i].Active = ok
		if ok {
			c.sessions[i].PaneID = info.PaneID
			c.sessions[i].PaneLabel = info.PaneLabel
		} else {
			c.sessions[i].PaneID = ""
			c.sessions[i].PaneLabel = ""
		}
	}
}
```

**Step 2: Test, commit**

---

### Task 12: Tmux integration

**Files:**
- Create: `daemon/tmux.go`

**Step 1: Implement GetActivePanes** — runs `tmux list-panes`, checks for claude processes.

```go
package daemon

import (
	"os/exec"
	"strings"
)

type PaneInfo struct {
	PaneID    string
	PaneLabel string
}

func GetActivePanes() map[string]PaneInfo {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{pane_id}|#{pane_pid}|#{pane_current_path}|#{session_name}:#{window_index}.#{pane_index}").Output()
	if err != nil {
		return nil
	}

	result := make(map[string]PaneInfo)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		paneID, panePID, panePath, paneLabel := parts[0], parts[1], parts[2], parts[3]
		if isClaude(panePID) {
			result[panePath] = PaneInfo{PaneID: paneID, PaneLabel: paneLabel}
		}
	}
	return result
}

func isClaude(pid string) bool {
	out, err := exec.Command("pgrep", "-P", pid, "-f", "claude").Output()
	if err == nil && len(out) > 0 {
		return true
	}
	out, err = exec.Command("ps", "-o", "command=", "-p", pid).Output()
	return err == nil && strings.Contains(string(out), "claude")
}

func TmuxSplitPane(dir, cmd string) error {
	return exec.Command("tmux", "split-window", "-h", "-c", dir, cmd).Run()
}

func TmuxNewWindow(dir, cmd string) error {
	return exec.Command("tmux", "new-window", "-c", dir, cmd).Run()
}

func TmuxSwitchToPane(paneLabel string) error {
	_ = exec.Command("tmux", "switch-client", "-t", paneLabel).Run()
	return exec.Command("tmux", "select-pane", "-t", paneLabel).Run()
}
```

**Step 2: Commit**

---

### Task 13: File watcher

**Files:**
- Create: `daemon/watcher.go`

**Step 1: Implement Watcher** — uses fsnotify to watch `~/.claude/` subdirectories, triggers cache reload on changes.

```go
package daemon

import (
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	watcher  *fsnotify.Watcher
	cache    *Cache
	debounce *time.Timer
}

func NewWatcher(cache *Cache, dirs Dirs) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch key directories
	for _, dir := range []string{dirs.Tasks, dirs.Todos, dirs.Plans} {
		_ = w.Add(dir)
	}
	// Watch history file's parent directory (for history.jsonl changes)
	_ = w.Add(filepath.Dir(dirs.History))

	return &Watcher{watcher: w, cache: cache}, nil
}

func (w *Watcher) Run() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				w.debouncedReload()
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)
		}
	}
}

func (w *Watcher) debouncedReload() {
	if w.debounce != nil {
		w.debounce.Stop()
	}
	w.debounce = time.AfterFunc(500*time.Millisecond, func() {
		if err := w.cache.Reload(); err != nil {
			log.Printf("reload error: %v", err)
		}
	})
}

func (w *Watcher) Close() {
	w.watcher.Close()
}
```

**Step 2: Commit**

---

### Task 14: Unix socket server

**Files:**
- Create: `daemon/server.go`

**Step 1: Implement Server** — listens on unix socket, handles client connections.

```go
package daemon

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"time"

	"cc-tui/protocol"
)

type Server struct {
	cache    *Cache
	sockPath string
	listener net.Listener
}

func NewServer(cache *Cache, sockPath string) *Server {
	return &Server{cache: cache, sockPath: sockPath}
}

func (s *Server) Start() error {
	// Clean up stale socket
	os.Remove(s.sockPath)

	ln, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return err
	}
	s.listener = ln

	go s.acceptLoop()

	// Periodic active pane polling
	go s.pollActivePanes()

	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	for {
		var req protocol.Request
		if err := dec.Decode(&req); err != nil {
			return
		}

		switch req.Cmd {
		case "tree":
			sessions := s.cache.Sessions()
			enc.Encode(protocol.Response{
				Type:     "snapshot",
				Sessions: sessions,
			})

		case "action":
			s.handleAction(req, enc)

		case "subscribe":
			// Hold connection open, send updates when cache changes
			s.streamUpdates(conn, enc)
			return
		}
	}
}

func (s *Server) handleAction(req protocol.Request, enc *json.Encoder) {
	sessions := s.cache.Sessions()
	var target *model.Session
	for i := range sessions {
		if sessions[i].ID == req.SessionID || sessions[i].Project == req.SessionID {
			target = &sessions[i]
			break
		}
	}
	if target == nil {
		enc.Encode(protocol.Response{Type: "error", Error: "session not found"})
		return
	}

	dir := target.Project
	claudeCmd := "claude --dangerously-skip-permissions"

	switch req.Action {
	case "open":
		if target.Active {
			TmuxSwitchToPane(target.PaneLabel)
		} else {
			TmuxSplitPane(dir, claudeCmd+" -r '"+target.ID+"'")
		}
	case "window":
		TmuxNewWindow(dir, claudeCmd+" -r '"+target.ID+"'")
	case "new":
		TmuxSplitPane(dir, claudeCmd)
	}

	enc.Encode(protocol.Response{Type: "ok"})
}

func (s *Server) pollActivePanes() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		panes := GetActivePanes()
		s.cache.UpdateActiveStatus(panes)
	}
}

func (s *Server) streamUpdates(conn net.Conn, enc *json.Encoder) {
	// Simple polling approach: send snapshot every 2s
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		sessions := s.cache.Sessions()
		if err := enc.Encode(protocol.Response{
			Type:     "update",
			Sessions: sessions,
		}); err != nil {
			return // client disconnected
		}
	}
}

func (s *Server) Close() {
	s.listener.Close()
	os.Remove(s.sockPath)
}
```

**Step 2: Commit**

---

### Task 15: Daemon entry point

**Files:**
- Create: `cmd/serve.go`

**Step 1: Implement RunServe**

```go
package cmd

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"cc-tui/daemon"
)

func RunServe() {
	home, _ := os.UserHomeDir()
	dirs := daemon.DefaultDirs()
	sockPath := filepath.Join(home, ".claude", "cc-tui.sock")
	logPath := filepath.Join(home, ".claude", "cc-tui.log")

	// Set up logging
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	log.Println("cc-tui daemon starting")

	cache := daemon.NewCache(dirs)
	if err := cache.Reload(); err != nil {
		log.Fatalf("initial load failed: %v", err)
	}
	log.Printf("loaded %d sessions", len(cache.Sessions()))

	watcher, err := daemon.NewWatcher(cache, dirs)
	if err != nil {
		log.Fatalf("watcher failed: %v", err)
	}
	defer watcher.Close()
	go watcher.Run()

	server := daemon.NewServer(cache, sockPath)
	if err := server.Start(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
	defer server.Close()

	log.Printf("listening on %s", sockPath)

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down")
}
```

**Step 2: Wire into main.go**

```go
func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		cmd.RunServe()
		return
	}
	cmd.RunClient()
}
```

**Step 3: Test daemon starts and responds**

Run: `go build -o cc-tui . && ./cc-tui serve &`

Then in another terminal:
```bash
echo '{"cmd":"tree"}' | socat - UNIX-CONNECT:~/.claude/cc-tui.sock
```
Expected: JSON response with sessions array

**Step 4: Commit**

---

## Phase 4: TUI — Bubble Tea Client

### Task 16: Styles

**Files:**
- Create: `tui/styles.go`

**Step 1: Define Lip Gloss styles**

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	ActiveDot   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))  // green
	InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // dim
	SessionName = lipgloss.NewStyle().Bold(true)
	TitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	PlanLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // blue
	DimStyle    = lipgloss.NewStyle().Faint(true)
	WIPStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	DoneStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("4")).
			Padding(0, 1)
	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")).
			Bold(true)
	HelpStyle = lipgloss.NewStyle().Faint(true)

	ProgressFull  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	ProgressEmpty = lipgloss.NewStyle().Faint(true)
)
```

**Step 2: Commit**

---

### Task 17: Tree component — node types and rendering

**Files:**
- Create: `tui/tree.go`

This is the core TUI component. Each node in the tree can be a session, category (Plan/Tasks/Todos), or leaf (step/task/todo).

**Step 1: Define node types**

```go
package tui

import "cc-tui/model"

type NodeKind int

const (
	NodeSession NodeKind = iota
	NodeCategory
	NodeLeaf
)

type TreeNode struct {
	Kind     NodeKind
	Label    string
	Session  *model.Session  // set for NodeSession
	Children []*TreeNode
	Expanded bool
	Depth    int
}
```

**Step 2: Implement BuildTree** — converts `[]model.Session` into a tree of `TreeNode`.

For each session:
- Create session node (expanded if active, collapsed otherwise)
- If has plan: category node "Plan: {title}" with step leaves
- If has tasks: category node "Tasks" with task leaves
- If has todos: category node "Todos" with todo leaves

**Step 3: Implement RenderNode** — renders a single node as a styled string.

- Session: `▶/▷ name  ●/○  time  [collapsed summary]`
- Category: `├─ Plan: title  3/8 ███░░`
- Leaf: `│  ☑/☐ text`

Use tree-drawing characters (├─, └─, │) for indentation.

**Step 4: Implement FlattenVisible** — returns the list of visible nodes (respecting collapsed state) for rendering and cursor navigation.

**Step 5: Commit**

---

### Task 18: Tree component — keyboard navigation

**Files:**
- Modify: `tui/tree.go`
- Create: `tui/keymap.go`

**Step 1: Define KeyMap**

```go
package tui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Open    key.Binding
	Window  key.Binding
	New     key.Binding
	Kill    key.Binding
	Filter  key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
	Top     key.Binding
	Bottom  key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "collapse")),
		Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "expand")),
		Open:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "open")),
		Window:  key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "window")),
		New:     key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		Kill:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "kill")),
		Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:    key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q", "quit")),
		Top:     key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
		Bottom:  key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
	}
}
```

**Step 2: Implement Update** for the tree model — handle each key binding. Left collapses or jumps to parent. Right expands. Enter finds the parent session node and dispatches open action.

**Step 3: Commit**

---

### Task 19: Mouse handling

**Files:**
- Modify: `tui/tree.go`

**Step 1: Handle tea.MouseMsg in Update**

```go
case tea.MouseMsg:
	switch msg.Button {
	case tea.MouseButtonLeft:
		// Single click: map Y coordinate to visible node, toggle expand
		clickedIdx := msg.Y - headerHeight + m.scrollOffset
		if clickedIdx >= 0 && clickedIdx < len(m.visible) {
			m.cursor = clickedIdx
			node := m.visible[clickedIdx]
			if len(node.Children) > 0 {
				node.Expanded = !node.Expanded
				m.rebuildVisible()
			}
		}
	}
```

**Step 2: Handle double-click** — track last click time. If two clicks within 400ms on same node → open session.

**Step 3: Handle Cmd-click** — check for `tea.KeyMsg` with alt modifier preceding the click (Bubble Tea reports modifier keys).

**Step 4: Commit**

---

### Task 20: Help overlay

**Files:**
- Create: `tui/help.go`

**Step 1: Implement help overlay** using `bubbles/help` component. Shows all keybindings in a formatted overlay when `?` is pressed.

**Step 2: Commit**

---

### Task 21: Root app model

**Files:**
- Create: `tui/app.go`

**Step 1: Implement the root Bubble Tea model**

```go
package tui

import (
	"encoding/json"
	"net"

	"cc-tui/model"
	"cc-tui/protocol"

	tea "github.com/charmbracelet/bubbletea"
)

type App struct {
	tree     TreeModel
	conn     net.Conn
	width    int
	height   int
	showHelp bool
	err      error
}

type sessionsMsg []model.Session
type errMsg error

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.fetchTree(),
		a.subscribeUpdates(),
		tea.EnableMouseCellMotion,
	)
}

func (a *App) fetchTree() tea.Cmd {
	return func() tea.Msg {
		enc := json.NewEncoder(a.conn)
		dec := json.NewDecoder(a.conn)
		enc.Encode(protocol.Request{Cmd: "tree"})
		var resp protocol.Response
		if err := dec.Decode(&resp); err != nil {
			return errMsg(err)
		}
		return sessionsMsg(resp.Sessions)
	}
}
```

**Step 2: Implement Update** — routes messages to tree model, handles window resize, help toggle, session actions (sends action requests to daemon over socket).

**Step 3: Implement View** — renders border, header, tree view, help footer.

```go
func (a App) View() string {
	header := HeaderStyle.Render(" CC Sessions ")
	tree := a.tree.View()
	help := HelpStyle.Render("↑↓ navigate  ←→ expand  ⏎ open  n new  q quit")

	if a.showHelp {
		// Render help overlay
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, tree, help)
}
```

**Step 4: Commit**

---

### Task 22: Client entry point

**Files:**
- Create: `cmd/client.go`

**Step 1: Implement RunClient**

```go
package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"cc-tui/tui"
)

func RunClient() {
	home, _ := os.UserHomeDir()
	sockPath := filepath.Join(home, ".claude", "cc-tui.sock")

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		// Auto-start daemon
		daemonBin, _ := os.Executable()
		cmd := exec.Command(daemonBin, "serve")
		cmd.Start()
		// Wait for socket
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			conn, err = net.Dial("unix", sockPath)
			if err == nil {
				break
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot connect to daemon: %v\n", err)
			os.Exit(1)
		}
	}
	defer conn.Close()

	app := tui.NewApp(conn)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 2: Commit**

---

## Phase 5: Integration

### Task 23: Build, test end-to-end

**Step 1: Build binary**

```bash
cd ~/.config/tmux/cc-tui && go build -o cc-tui .
```

**Step 2: Start daemon manually, verify logs**

```bash
./cc-tui serve &
cat ~/.claude/cc-tui.log
```
Expected: "loaded N sessions", "listening on ..."

**Step 3: Run client, verify tree renders**

```bash
./cc-tui
```
Expected: tree view with sessions, navigate with arrows, expand/collapse, quit with q

**Step 4: Test session actions** — Enter on active session switches pane, Enter on inactive resumes, n creates new, w opens window.

**Step 5: Commit**

---

### Task 24: Toggle script and tmux config

**Files:**
- Create: `~/.config/tmux/cc-tui-toggle.sh`
- Modify: `~/.config/tmux/tmux.conf.local`

**Step 1: Create toggle script**

```bash
#!/bin/sh
PANE_ID=$(tmux list-panes -a -F '#{pane_id} #{pane_title}' \
  | grep 'cc-tui' | head -1 | cut -d' ' -f1)
if [ -n "$PANE_ID" ]; then
  tmux kill-pane -t "$PANE_ID"
else
  tmux split-window -h -l 45 \
    "tmux select-pane -T cc-tui; ~/.config/tmux/cc-tui/cc-tui"
fi
```

```bash
chmod +x ~/.config/tmux/cc-tui-toggle.sh
```

**Step 2: Update tmux.conf.local** — point User9 (Cmd+D) to new toggle:

```
bind -n User9 run-shell '~/.config/tmux/cc-tui-toggle.sh'
```

Remove old Cmd+S session manager bindings (User0 freed).

**Step 3: Reload tmux config and test Cmd+D**

```bash
tmux source-file ~/.config/tmux/tmux.conf.local
```

**Step 4: Commit**

---

### Task 25: Launchd plist

**Files:**
- Create: `~/Library/LaunchAgents/com.cc-tui.daemon.plist`

**Step 1: Create plist**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.cc-tui.daemon</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Users/allan.beihl/.config/tmux/cc-tui/cc-tui</string>
    <string>serve</string>
  </array>
  <key>KeepAlive</key>
  <true/>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/Users/allan.beihl/.claude/cc-tui-stdout.log</string>
  <key>StandardErrorPath</key>
  <string>/Users/allan.beihl/.claude/cc-tui-stderr.log</string>
</dict>
</plist>
```

**Step 2: Load and verify**

```bash
launchctl load ~/Library/LaunchAgents/com.cc-tui.daemon.plist
launchctl list | grep cc-tui
```

**Step 3: Commit**

---

## Phase 6: Migration

### Task 26: Remove old files

**Step 1: Remove replaced files**

```bash
rm ~/.config/tmux/cc-sessions-core.sh
rm ~/.config/tmux/cc-sessions.sh
rm ~/.config/tmux/cc-sessions-dock.sh
rm ~/.config/tmux/cc-session-data.py
rm ~/.config/tmux/cc-dashboard.py
rm ~/.config/tmux/cc-dashboard-run.sh
rm ~/.config/tmux/cc-dashboard-toggle.sh
rm ~/.config/tmux/cc-dock-toggle.sh
```

**Step 2: Update pane-menu.sh** — remove references to old scripts.

**Step 3: Clean up tmux.conf.local** — remove old User0 bindings for session manager.

**Step 4: Final verification** — Cmd+D toggles cc-tui, all session actions work, daemon auto-restarts.

**Step 5: Commit**

```bash
git commit -m "chore: remove old fzf/python session manager and dashboard"
```
