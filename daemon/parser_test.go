package daemon

import (
	"cc-tui/model"
	"os"
	"path/filepath"
	"testing"
)

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
	// LastDisplay should hold the most recent prompt text per session.
	var abc *HistoryEntry
	for i := range sessions {
		if sessions[i].ID == "abc-123" {
			abc = &sessions[i]
		}
	}
	if abc == nil {
		t.Fatal("abc-123 missing from history")
	}
	if abc.LastDisplay != "most recent prompt for abc-123" {
		t.Errorf("LastDisplay = %q, want most recent prompt", abc.LastDisplay)
	}
}

func TestLoadSessionMeta(t *testing.T) {
	meta := LoadSessionMeta("testdata/session.jsonl")
	if meta.Slug != "test-slug" {
		t.Errorf("slug = %q, want test-slug", meta.Slug)
	}
	if meta.Title != "My Test Session" {
		t.Errorf("title = %q, want My Test Session", meta.Title)
	}
	if meta.LastUserMsg != "Implement the REST API endpoints for the user service" {
		t.Errorf("lastUserMsg = %q, want 'Implement the REST API...'", meta.LastUserMsg)
	}
	if meta.GitBranch != "feat/user-api" {
		t.Errorf("gitBranch = %q, want feat/user-api", meta.GitBranch)
	}
	// Context size = sum of the last assistant message's usage fields.
	if meta.ContextTokens != 52500 {
		t.Errorf("contextTokens = %d, want 52500", meta.ContextTokens)
	}
}

func TestLoadTasks(t *testing.T) {
	tasks := LoadTasks("test-uuid-123", "testdata/tasks")
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	found := false
	for _, task := range tasks {
		if task.Subject == "Set up project structure" && task.Status == "completed" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find 'Set up project structure' task with status completed")
	}
}

func TestLoadTodos(t *testing.T) {
	todos := LoadTodos("test-uuid-123", "testdata/todos")
	if len(todos) != 3 {
		t.Fatalf("got %d todos, want 3", len(todos))
	}
	if todos[0].Content != "Search codebase for existing references" {
		t.Errorf("first todo content = %q", todos[0].Content)
	}
	if todos[0].Status != "completed" {
		t.Errorf("first todo status = %q, want completed", todos[0].Status)
	}
}

func TestLoadPlan(t *testing.T) {
	plan := LoadPlan("testdata/sample-plan.md")
	if plan == nil {
		t.Fatal("plan is nil")
	}
	if plan.Title != "Build a REST API Service" {
		t.Errorf("title = %q, want 'Build a REST API Service'", plan.Title)
	}
	if len(plan.Steps) != 5 {
		t.Fatalf("got %d steps, want 5", len(plan.Steps))
	}
	// Verify bold/code stripping
	if plan.Steps[1].Text != "Implement database layer" {
		t.Errorf("step 2 text = %q, want 'Implement database layer'", plan.Steps[1].Text)
	}
}

func TestLoadPlanTable(t *testing.T) {
	plan := LoadPlan("testdata/sample-plan-table.md")
	if plan == nil {
		t.Fatal("plan is nil")
	}
	if plan.Title != "Migration Plan" {
		t.Errorf("title = %q", plan.Title)
	}
	if len(plan.Steps) != 4 {
		t.Fatalf("got %d steps, want 4", len(plan.Steps))
	}
}

func TestLoadPlanFallback(t *testing.T) {
	plan := LoadPlan("testdata/sample-plan-fallback.md")
	if plan == nil {
		t.Fatal("plan is nil")
	}
	if plan.Title != "Refactor Authentication" {
		t.Errorf("title = %q", plan.Title)
	}
	// Should skip "Context", "Key Insight", "Verification" and pick up 3 actionable headings
	if len(plan.Steps) != 3 {
		t.Fatalf("got %d steps, want 3: %+v", len(plan.Steps), plan.Steps)
	}
}

func TestLoadPlanReal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real file test")
	}
	plans, _ := filepath.Glob(os.ExpandEnv("$HOME/.claude/plans/*.md"))
	for _, p := range plans {
		plan := LoadPlan(p)
		if plan != nil {
			t.Logf("%s: title=%q steps=%d", filepath.Base(p), plan.Title, len(plan.Steps))
		} else {
			t.Logf("%s: nil plan", filepath.Base(p))
		}
	}
}

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

func TestCleanMessage(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"<system>Hello</system> world toolu_abc123", "Hello world"},
		{"Fix the 8a3b4c5d commit issue", "Fix the commit issue"},
		{"hi", ""},
		{"Implement the REST API endpoints for the user service", "Implement the REST API endpoints for the user service"},
		{"Set model to \x1b[1mOpus 4.8\x1b[22m now", "Set model to Opus 4.8 now"},
	}
	for _, tt := range tests {
		got := CleanMessage(tt.input)
		if got != tt.want {
			t.Errorf("CleanMessage(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLoadFullSessionReal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dirs := DefaultDirs()
	entries, _ := LoadHistory(dirs.History)
	limit := 3
	if len(entries) < limit {
		limit = len(entries)
	}
	for _, e := range entries[:limit] {
		s := LoadFullSession(e, dirs)
		t.Logf("%s: title=%q plan=%v tasks=%d todos=%d",
			s.DirName, s.Title, s.Plan != nil, len(s.Tasks), len(s.Todos))
	}
}
