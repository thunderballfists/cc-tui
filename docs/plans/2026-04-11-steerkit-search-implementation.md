# SteerKit Deep Search Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add semantic search to cc-tui that queries SteerKit's `/recall` API and lets users jump to matching sessions in the tree.

**Architecture:** The TUI client makes HTTP calls directly to SteerKit (`127.0.0.1:7419`) — the cc-tui daemon is not involved. Search is an overlay mode following the same pattern as preview and filter. A startup health probe enables/disables the feature.

**Tech Stack:** Go `net/http` for SteerKit API calls, Bubble Tea for UI, existing lipgloss styles.

---

### Task 1: Add Search key binding

**Files:**
- Modify: `tui/keymap.go:5-19` (KeyMap struct)
- Modify: `tui/keymap.go:22-39` (DefaultKeyMap)
- Modify: `tui/keymap.go:47-53` (FullHelp)

**Step 1: Add Search field to KeyMap struct**

In `tui/keymap.go`, add `Search` to the struct after `Bottom`:

```go
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
	Search  key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
	Top     key.Binding
	Bottom  key.Binding
}
```

**Step 2: Add binding in DefaultKeyMap**

After the `Filter` line in `DefaultKeyMap()`:

```go
Search:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "search")),
```

**Step 3: Add Search to FullHelp**

In the third group of `FullHelp()`, add `k.Search`:

```go
{k.Filter, k.Search, k.Refresh, k.Help, k.Quit},
```

**Step 4: Verify it compiles**

Run: `cd /Users/allan.beihl/.config/tmux/cc-tui && go build ./tui/...`
Expected: success (no errors)

**Step 5: Commit**

```bash
git add tui/keymap.go
git commit -m "feat: add Search key binding (s) to keymap"
```

---

### Task 2: Create search types and state

**Files:**
- Create: `tui/search.go`

**Step 1: Create tui/search.go with types and constructor**

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// SearchResult is a single result from SteerKit /recall.
type SearchResult struct {
	SourceType string    // "exchange" or "episode"
	SessionID  string
	Score      float64
	Summary    string
	Project    string
	Timestamp  time.Time
}

// SearchState manages the deep search overlay.
type SearchState struct {
	input     textinput.Model
	results   []SearchResult
	cursor    int
	offset    int
	querying  bool
	noResults bool
	width     int
	height    int
}

// NewSearchState returns an initialized SearchState.
func NewSearchState() SearchState {
	ti := textinput.New()
	ti.Prompt = "search: "
	ti.CharLimit = 80
	return SearchState{input: ti}
}

func (s *SearchState) SetSize(w, h int) {
	s.width = w
	s.height = h
}

func (s *SearchState) Reset() {
	s.input.SetValue("")
	s.results = nil
	s.cursor = 0
	s.offset = 0
	s.querying = false
	s.noResults = false
}

func (s *SearchState) contentHeight() int {
	h := s.height - 3 // banner + footer + input
	if h < 1 {
		h = 1
	}
	return h
}

// View renders the search results list.
func (s *SearchState) View() string {
	if s.querying {
		return DimStyle.Render("  Searching...")
	}
	if s.noResults {
		return DimStyle.Render(fmt.Sprintf("  No results for '%s'", s.input.Value()))
	}
	if len(s.results) == 0 {
		return DimStyle.Render("  Type a query and press Enter")
	}

	viewHeight := s.contentHeight()
	var lines []string

	end := s.offset + viewHeight
	if end > len(s.results) {
		end = len(s.results)
	}

	for i := s.offset; i < end; i++ {
		r := s.results[i]
		selected := i == s.cursor

		// Type indicator
		typeInd := "C"
		if r.SourceType == "episode" {
			typeInd = "E"
		}

		// Score
		score := DimStyle.Render(fmt.Sprintf("%.2f", r.Score))

		// Summary — truncate to fit
		summary := r.Summary
		maxSummary := s.width - 30
		if maxSummary < 20 {
			maxSummary = 20
		}
		if len(summary) > maxSummary {
			summary = summary[:maxSummary] + "…"
		}

		// Project
		project := DimStyle.Render(r.Project)

		// Time
		ts := relativeTime(r.Timestamp)

		line := fmt.Sprintf(" %s  %s  %s  %s  %s",
			score,
			PlanLabel.Render(typeInd),
			summary,
			project,
			DimStyle.Render(ts),
		)

		if selected {
			line = CursorStyle.Render("▶") + line
		} else {
			line = " " + line
		}

		if s.width > 0 {
			line = lipgloss.NewStyle().MaxWidth(s.width).Render(line)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/allan.beihl/.config/tmux/cc-tui && go build ./tui/...`
Expected: success

**Step 3: Commit**

```bash
git add tui/search.go
git commit -m "feat: add SearchResult type and SearchState with rendering"
```

---

### Task 3: Add SteerKit HTTP client and health probe

**Files:**
- Create: `tui/steerkit.go`

**Step 1: Create tui/steerkit.go**

This file handles all HTTP communication with the SteerKit daemon. The port is read from `STEERKIT_DAEMON_PORT` env var, defaulting to `7419`.

```go
package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var steerKitClient = &http.Client{Timeout: 3 * time.Second}

func steerKitBaseURL() string {
	port := os.Getenv("STEERKIT_DAEMON_PORT")
	if port == "" {
		port = "7419"
	}
	return fmt.Sprintf("http://127.0.0.1:%s", port)
}

// steerKitAvailableMsg is sent after the startup health probe.
type steerKitAvailableMsg struct{ available bool }

// checkSteerKitCmd probes GET /health to see if SteerKit is running.
func checkSteerKitCmd() tea.Msg {
	resp, err := steerKitClient.Get(steerKitBaseURL() + "/health")
	if err != nil {
		return steerKitAvailableMsg{false}
	}
	resp.Body.Close()
	return steerKitAvailableMsg{resp.StatusCode == 200}
}

// recallResult maps the relevant fields from GET /recall response.
type recallResult struct {
	SessionID  string `json:"session_id"`
	SourceType string `json:"source_type"`
	Score      float64 `json:"score"`
	Summary    string `json:"summary"`
	Detail     string `json:"detail"`
	Project    string `json:"project"`
	Timestamp  string `json:"timestamp"`
	// episode fields
	Goal    string `json:"goal"`
	Outcome string `json:"outcome"`
}

type recallResponse struct {
	Results []recallResult `json:"results"`
}

// searchResultsMsg carries parsed results back to the App.
type searchResultsMsg struct {
	results []SearchResult
}

// doRecallSearch queries GET /recall and returns searchResultsMsg.
func doRecallSearch(query string) tea.Cmd {
	return func() tea.Msg {
		u := fmt.Sprintf("%s/recall?q=%s&limit=20", steerKitBaseURL(), url.QueryEscape(query))
		resp, err := steerKitClient.Get(u)
		if err != nil {
			return searchResultsMsg{nil}
		}
		defer resp.Body.Close()

		var body recallResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return searchResultsMsg{nil}
		}

		var results []SearchResult
		for _, r := range body.Results {
			// Only include exchanges and episodes (they have session_id)
			if r.SourceType != "exchange" && r.SourceType != "episode" {
				continue
			}
			if r.SessionID == "" {
				continue
			}
			summary := r.Summary
			if summary == "" && r.SourceType == "episode" {
				summary = r.Goal
			}
			ts, _ := time.Parse(time.RFC3339, r.Timestamp)
			results = append(results, SearchResult{
				SourceType: r.SourceType,
				SessionID:  r.SessionID,
				Score:      r.Score,
				Summary:    summary,
				Project:    r.Project,
				Timestamp:  ts,
			})
		}
		return searchResultsMsg{results}
	}
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/allan.beihl/.config/tmux/cc-tui && go build ./tui/...`
Expected: success

**Step 3: Commit**

```bash
git add tui/steerkit.go
git commit -m "feat: add SteerKit HTTP client with health probe and /recall search"
```

---

### Task 4: Wire search mode into App

**Files:**
- Modify: `tui/app.go:21-38` (App struct)
- Modify: `tui/app.go:48-65` (NewApp)
- Modify: `tui/app.go:67-73` (Init)
- Modify: `tui/app.go:177-373` (Update)
- Modify: `tui/app.go:375-403` (View)

**Step 1: Add fields to App struct**

Add after the `filterText` field:

```go
steerKitAvailable bool
showSearch        bool
search            SearchState
```

**Step 2: Initialize search state in NewApp**

In the `return &App{` block, add:

```go
search: NewSearchState(),
```

**Step 3: Add health probe to Init**

Change Init to include the health check:

```go
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.fetchTree(),
		tea.EnableMouseCellMotion,
		tickCmd(),
		checkSteerKitCmd,
	)
}
```

**Step 4: Handle steerKitAvailableMsg in Update**

Add a case in the `switch msg := msg.(type)` block:

```go
case steerKitAvailableMsg:
	a.steerKitAvailable = msg.available
	return a, nil
```

**Step 5: Handle searchResultsMsg in Update**

Add a case:

```go
case searchResultsMsg:
	a.search.querying = false
	if len(msg.results) == 0 {
		a.search.noResults = true
	} else {
		a.search.results = msg.results
		a.search.cursor = 0
		a.search.offset = 0
	}
	return a, nil
```

**Step 6: Add search overlay input handling in the tea.KeyMsg section**

After the preview overlay block and before the filter mode block (after line 310, before line 312), add search mode handling:

```go
// Search mode
if a.showSearch {
	switch msg.String() {
	case "esc":
		a.showSearch = false
		a.search.Reset()
		return a, nil
	case "enter":
		// If we have results and cursor is on one, jump to it
		if len(a.search.results) > 0 {
			r := a.search.results[a.search.cursor]
			a.showSearch = false
			// Find and expand the project containing this session
			found := false
			for _, g := range a.groups {
				for _, s := range g.Sessions {
					if s.ID == r.SessionID {
						a.expandState["p:"+g.Project] = true
						a.filterText = ""
						a.filter.SetValue("")
						filtered := a.applyFilter(a.groups)
						a.tree.SetGroups(filtered, a.expandState)
						a.tree.ScrollToSession(r.SessionID)
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			a.search.Reset()
			return a, nil
		}
		// No results yet — treat Enter as submit query
		query := a.search.input.Value()
		if query != "" {
			a.search.querying = true
			a.search.noResults = false
			a.search.results = nil
			return a, doRecallSearch(query)
		}
		return a, nil
	case "up", "k":
		if len(a.search.results) > 0 && a.search.cursor > 0 {
			a.search.cursor--
			if a.search.cursor < a.search.offset {
				a.search.offset = a.search.cursor
			}
		}
		return a, nil
	case "down", "j":
		if len(a.search.results) > 0 && a.search.cursor < len(a.search.results)-1 {
			a.search.cursor++
			viewH := a.search.contentHeight()
			if a.search.cursor >= a.search.offset+viewH {
				a.search.offset = a.search.cursor - viewH + 1
			}
		}
		return a, nil
	default:
		// Forward to text input when no results yet
		if len(a.search.results) == 0 && !a.search.querying {
			var cmd tea.Cmd
			a.search.input, cmd = a.search.input.Update(msg)
			return a, cmd
		}
		return a, nil
	}
}
```

**Step 7: Add 's' key to enter search mode**

After the filter key check (line 346-350) and before the 'p' preview check, add:

```go
if key.Matches(msg, a.tree.keys.Search) && a.steerKitAvailable {
	a.showSearch = true
	a.search.Reset()
	a.search.SetSize(a.width, a.height)
	a.search.input.Focus()
	return a, a.search.input.Cursor.BlinkCmd()
}
```

**Step 8: Update View to render search**

In `View()`, after the help overlay check and before the preview overlay check, add:

```go
if a.showSearch {
	header := renderBanner(a.width, a.groups, a.filterText, a.err != nil, a.tick)
	var footer string
	if len(a.search.results) == 0 && !a.search.querying {
		footer = a.search.input.View()
	} else {
		footer = HelpStyle.Render("  ↑↓ navigate  ⏎ select  esc cancel")
	}
	a.search.SetSize(a.width, a.height)
	return lipgloss.JoinVertical(lipgloss.Left, header, a.search.View(), footer)
}
```

**Step 9: Update footer help text**

In the `View()` else branch for the footer, update the help text to include `s search` when available:

```go
if a.steerKitAvailable {
	footer = HelpStyle.Render("  ↑↓ navigate  ←→ expand  ⏎ open  n new  / filter  s search  p preview  ? help  q quit")
} else {
	footer = HelpStyle.Render("  ↑↓ navigate  ←→ expand  ⏎ open  n new  / filter  p preview  ? help  q quit")
}
```

**Step 10: Verify it compiles**

Run: `cd /Users/allan.beihl/.config/tmux/cc-tui && go build ./tui/...`
Expected: will fail — `ScrollToSession` doesn't exist yet. That's expected — we add it in Task 5.

---

### Task 5: Add ScrollToSession to TreeModel

**Files:**
- Modify: `tui/model.go` (add method after `jumpToParent`)

**Step 1: Add ScrollToSession method**

After `jumpToParent()` (line 228), add:

```go
// ScrollToSession finds a session by ID in the visible nodes,
// sets the cursor to it, and adjusts scroll offset to center it.
// Returns false if the session is not in the visible list.
func (m *TreeModel) ScrollToSession(sessionID string) bool {
	for i, node := range m.visible {
		if node.Kind == NodeSnapshot && node.Session != nil && node.Session.ID == sessionID {
			m.cursor = i
			// Center in viewport
			viewHeight := m.height - 3
			if viewHeight < 1 {
				viewHeight = 1
			}
			m.scrollOffset = m.cursor - viewHeight/2
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			maxOffset := len(m.visible) - viewHeight
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.scrollOffset > maxOffset {
				m.scrollOffset = maxOffset
			}
			return true
		}
	}
	return false
}
```

**Step 2: Verify everything compiles**

Run: `cd /Users/allan.beihl/.config/tmux/cc-tui && go build -o cc-tui .`
Expected: success — full binary builds

**Step 3: Commit**

```bash
git add tui/model.go tui/app.go
git commit -m "feat: wire search mode into App with jump-to-session"
```

---

### Task 6: Add banner indicator for SteerKit availability

**Files:**
- Modify: `tui/banner.go:25` (renderBanner signature — add steerKit param)
- Modify: `tui/banner.go:117` (buildRightStatus — add indicator)
- Modify: `tui/app.go` (pass steerKitAvailable to renderBanner)

**Step 1: Add steerKit parameter to renderBanner**

Change signature:

```go
func renderBanner(width int, groups []model.ProjectGroup, filterText string, hasErr bool, tick int, steerKit bool) string {
```

**Step 2: Pass steerKit to buildRightStatus**

Change the call:

```go
right := buildRightStatus(width-titleLen-2, activeCount, len(groups), totalSessions, filterText, steerKit)
```

**Step 3: Add steerKit param to buildRightStatus**

Change signature:

```go
func buildRightStatus(avail, active, projects, sessions int, filter string, steerKit bool) string {
```

Add a search indicator prefix. Before the `if filter != ""` block:

```go
skIndicator := ""
if steerKit {
	skIndicator = "⚡"
}
```

Then prepend `skIndicator` to each candidate string. For the filter case:

```go
if filter != "" {
	candidates = append(candidates,
		fmt.Sprintf(" %s🔍%s ◆ ● %d ◆ %dp %ds ", skIndicator, filter, active, projects, sessions),
	)
}
```

For the active > 0 candidates, prepend to the first one:

```go
fmt.Sprintf(" %s● %d active %s %d proj %s %d sess ", skIndicator, active, div, projects, div, sessions),
```

And for the inactive first candidate:

```go
fmt.Sprintf(" %s%d proj %s %d sess ", skIndicator, projects, div, sessions),
```

**Step 4: Update all renderBanner call sites**

In `app.go` View():

```go
header := renderBanner(a.width, a.groups, a.filterText, a.err != nil, a.tick, a.steerKitAvailable)
```

There are two call sites — one in the main View() and one in the search overlay View section added in Task 4. Update both.

**Step 5: Verify it compiles**

Run: `cd /Users/allan.beihl/.config/tmux/cc-tui && go build -o cc-tui .`
Expected: success

**Step 6: Commit**

```bash
git add tui/banner.go tui/app.go
git commit -m "feat: show SteerKit availability indicator in banner"
```

---

### Task 7: Manual testing

**Step 1: Build and run**

```bash
cd /Users/allan.beihl/.config/tmux/cc-tui && go build -o cc-tui .
```

**Step 2: Test with SteerKit running**

Launch cc-tui inside tmux. Verify:
- Banner shows `⚡` indicator
- Footer shows `s search`
- Pressing `s` opens search input
- Typing a query and pressing Enter shows results (or "No results")
- Up/down navigates results
- Enter on a result jumps to the session in the tree
- Esc returns to tree

**Step 3: Test with SteerKit not running**

Stop SteerKit daemon, relaunch cc-tui. Verify:
- No `⚡` in banner
- No `s search` in footer
- Pressing `s` does nothing

**Step 4: Commit the final build**

```bash
git add -A
git commit -m "feat: SteerKit deep search integration"
```
