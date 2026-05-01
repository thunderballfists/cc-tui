package tui

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cc-tui/model"
	"cc-tui/protocol"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type App struct {
	tree        TreeModel
	groups      []model.ProjectGroup
	sockPath    string
	mu          sync.Mutex
	width       int
	height      int
	showHelp    bool
	showPreview bool
	preview     PreviewState
	filtering   bool
	filter      textinput.Model
	filterText  string
	steerKitAvailable  bool
	sessionSummaries   map[string]string // sessionID → summary from SteerKit
	showSearch         bool
	search             SearchState
	err            error
	expandState    map[string]bool
	tick           int
	lastActionTime time.Time
}

type convMsg []model.ConvMessage

type groupsMsg []model.ProjectGroup
type errMsg struct{ err error }
type tickMsg time.Time

func (e errMsg) Error() string { return e.err.Error() }

func NewApp(conn net.Conn) *App {
	conn.Close()

	home, _ := os.UserHomeDir()
	sockPath := filepath.Join(home, ".claude", "cc-tui.sock")

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.CharLimit = 40

	return &App{
		tree:        NewTreeModel(),
		sockPath:    sockPath,
		expandState: make(map[string]bool),
		showPreview: false,
		filter:      fi,
		search:      NewSearchState(),
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.fetchTree(),
		tea.EnableMouseCellMotion,
		tickCmd(),
		checkSteerKitCmd,
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a *App) doRequest(req protocol.Request) (*protocol.Response, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	conn, err := net.DialTimeout("unix", a.sockPath, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)

	if err := enc.Encode(req); err != nil {
		return nil, err
	}

	var resp protocol.Response
	if err := dec.Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (a *App) fetchTree() tea.Cmd {
	return func() tea.Msg {
		resp, err := a.doRequest(protocol.Request{Cmd: "tree"})
		if err != nil {
			return errMsg{err}
		}
		return groupsMsg(resp.Groups)
	}
}

func (a *App) fetchConversation(sessionID string) tea.Cmd {
	return func() tea.Msg {
		resp, err := a.doRequest(protocol.Request{Cmd: "conversation", SessionID: sessionID})
		if err != nil {
			return errMsg{err}
		}
		return convMsg(resp.Conversation)
	}
}

func (a *App) sendAction(action, sessionID, project string) tea.Cmd {
	return func() tea.Msg {
		// Use session ID if available (specific snapshot), fall back to project path
		targetID := sessionID
		if targetID == "" {
			targetID = project
		}
		resp, err := a.doRequest(protocol.Request{
			Cmd:       "action",
			Action:    action,
			SessionID: targetID,
		})
		if err != nil {
			return errMsg{err}
		}
		if resp.Type == "error" && targetID != project {
			a.doRequest(protocol.Request{
				Cmd:       "action",
				Action:    action,
				SessionID: project,
			})
		}
		return nil
	}
}

func (a *App) applyFilter(groups []model.ProjectGroup) []model.ProjectGroup {
	if a.filterText == "" {
		return groups
	}
	query := strings.ToLower(a.filterText)
	var filtered []model.ProjectGroup
	for _, g := range groups {
		if strings.Contains(strings.ToLower(g.DirName), query) ||
			strings.Contains(strings.ToLower(g.Project), query) {
			filtered = append(filtered, g)
			continue
		}
		// Check session titles/messages
		for _, s := range g.Sessions {
			if strings.Contains(strings.ToLower(s.Title), query) ||
				strings.Contains(strings.ToLower(s.LastMsg), query) {
				filtered = append(filtered, g)
				break
			}
		}
	}
	return filtered
}

// jumpToSession expands the project containing sessionID and scrolls to it.
func (a *App) jumpToSession(sessionID string) bool {
	for _, g := range a.groups {
		for _, s := range g.Sessions {
			if s.ID == sessionID {
				a.expandState["p:"+g.Project] = true
				a.filterText = ""
				a.filter.SetValue("")
				filtered := a.applyFilter(a.groups)
				a.tree.SetGroups(filtered, a.expandState)
				a.tree.ScrollToSession(sessionID)
				return true
			}
		}
	}
	return false
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.tree.SetSize(msg.Width-2, msg.Height)
		a.preview.SetSize(msg.Width, msg.Height)
		return a, nil

	case groupsMsg:
		a.err = nil
		a.groups = []model.ProjectGroup(msg)
		// Enrich sessions with SteerKit summaries
		if len(a.sessionSummaries) > 0 {
			for i := range a.groups {
				for j := range a.groups[i].Sessions {
					if s, ok := a.sessionSummaries[a.groups[i].Sessions[j].ID]; ok {
						a.groups[i].Sessions[j].Summary = s
					}
				}
			}
		}
		filtered := a.applyFilter(a.groups)
		a.tree.SetGroups(filtered, a.expandState)
		return a, nil

	case tickMsg:
		a.tick++
		return a, tea.Batch(a.fetchTree(), tickCmd())

	case convMsg:
		a.preview.SetMessages([]model.ConvMessage(msg))
		return a, nil

	case steerKitAvailableMsg:
		a.steerKitAvailable = msg.available
		if msg.available {
			return a, fetchSessionSummariesCmd
		}
		return a, nil

	case sessionSummariesMsg:
		if msg.summaries != nil {
			a.sessionSummaries = msg.summaries
		}
		return a, nil

	case searchResultsMsg:
		a.search.querying = false
		if len(msg.results) == 0 {
			a.search.noResults = true
		} else {
			// Enrich results with session summaries
			for i, r := range msg.results {
				if summary, ok := a.sessionSummaries[r.SessionID]; ok {
					msg.results[i].SessionSummary = summary
				}
			}
			a.search.results = msg.results
			a.search.cursor = 0
			a.search.offset = 0
		}
		return a, nil

	case errMsg:
		a.err = msg.err
		return a, nil

	case ActionMsg:
		// Debounce — ignore actions within 1s of last one
		if time.Since(a.lastActionTime) < time.Second {
			return a, nil
		}
		a.lastActionTime = time.Now()
		return a, a.sendAction(msg.Action, msg.SessionID, msg.Project)

	case RefreshMsg:
		return a, a.fetchTree()

	case tea.MouseMsg:
		if a.showPreview {
			switch msg.Type {
			case tea.MouseWheelUp:
				a.preview.HandleMouse(msg.Y, true, true)
			case tea.MouseWheelDown:
				a.preview.HandleMouse(msg.Y, true, false)
			case tea.MouseLeft:
				if msg.X >= a.width-5 {
					a.preview.dragging = true
					a.preview.HandleMouse(msg.Y, false, false)
				}
			case tea.MouseMotion:
				if a.preview.dragging {
					a.preview.HandleMouse(msg.Y, false, false)
				}
			case tea.MouseRelease:
				a.preview.dragging = false
			}
			return a, nil
		}

	case tea.KeyMsg:
		// Help overlay dismissal
		if a.showHelp {
			a.showHelp = false
			return a, nil
		}

		// Preview overlay
		if a.showPreview {
			// Search mode within preview
			if a.preview.searching {
				switch msg.String() {
				case "enter", "esc":
					a.preview.searching = false
					if msg.String() == "esc" {
						a.preview.search = ""
					}
					return a, nil
				case "backspace":
					if len(a.preview.search) > 0 {
						a.preview.search = a.preview.search[:len(a.preview.search)-1]
						a.preview.scroll = 0
					}
					return a, nil
				default:
					if len(msg.String()) == 1 {
						a.preview.search += msg.String()
						a.preview.scroll = 0
					}
					return a, nil
				}
			}

			switch msg.String() {
			case "p", "esc":
				a.showPreview = false
				return a, nil
			case "q":
				return a, tea.Quit
			case "j", "down":
				a.preview.ScrollDown(3)
				return a, nil
			case "k", "up":
				a.preview.ScrollUp(3)
				return a, nil
			case "d":
				a.preview.ScrollDown(a.height / 2)
				return a, nil
			case "u":
				a.preview.ScrollUp(a.height / 2)
				return a, nil
			case "g", "home":
				a.preview.scroll = 0
				return a, nil
			case "G", "end":
				a.preview.ScrollDown(99999)
				return a, nil
			case " ":
				a.preview.ScrollDown(a.height / 2)
				return a, nil
			case "/":
				a.preview.searching = true
				a.preview.search = ""
				return a, nil
			case "enter":
				var cmd tea.Cmd
				a.tree, cmd = a.tree.Update(msg)
				a.tree.SaveExpandState(a.expandState)
				a.showPreview = false
				return a, cmd
			}
			return a, nil
		}

		// Search mode
		if a.showSearch {
			// Input mode: all keys go to text input except esc/enter
			if len(a.search.results) == 0 && !a.search.querying {
				switch msg.String() {
				case "esc":
					if a.search.noResults {
						a.search.noResults = false
						return a, nil
					}
					a.showSearch = false
					a.search.Reset()
					return a, nil
				case "enter":
					query := a.search.input.Value()
					if query != "" {
						a.search.querying = true
						a.search.noResults = false
						return a, doRecallSearch(query)
					}
					return a, nil
				default:
					a.search.noResults = false
					var cmd tea.Cmd
					a.search.input, cmd = a.search.input.Update(msg)
					return a, cmd
				}
			}
			// Results mode: j/k navigate, esc/slash go back to input
			switch msg.String() {
			case "esc", "/":
				a.search.ClearResults()
				a.search.input.Focus()
				return a, a.search.input.Cursor.BlinkCmd()
			case "enter":
				if len(a.search.results) > 0 {
					r := a.search.results[a.search.cursor]
					a.showSearch = false
					found := a.jumpToSession(r.SessionID)
					if !found {
						a.err = fmt.Errorf("session not in cache")
					}
					a.search.Reset()
					return a, nil
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
			}
			return a, nil
		}

		// Filter mode
		if a.filtering {
			switch msg.String() {
			case "enter", "esc":
				a.filtering = false
				a.filter.Blur()
				if msg.String() == "esc" {
					a.filterText = ""
					a.filter.SetValue("")
				} else {
					a.filterText = a.filter.Value()
				}
				filtered := a.applyFilter(a.groups)
				a.tree.SetGroups(filtered, a.expandState)
				return a, nil
			default:
				var cmd tea.Cmd
				a.filter, cmd = a.filter.Update(msg)
				// Live filter as you type
				a.filterText = a.filter.Value()
				filtered := a.applyFilter(a.groups)
				a.tree.SetGroups(filtered, a.expandState)
				return a, cmd
			}
		}

		// Global keys
		if key.Matches(msg, a.tree.keys.Help) {
			a.showHelp = !a.showHelp
			return a, nil
		}
		if key.Matches(msg, a.tree.keys.Quit) {
			return a, tea.Quit
		}
		if key.Matches(msg, a.tree.keys.Filter) {
			a.filtering = true
			a.filter.Focus()
			return a, a.filter.Cursor.BlinkCmd()
		}
		if key.Matches(msg, a.tree.keys.Search) && a.steerKitAvailable {
			a.showSearch = true
			a.search.Reset()
			a.search.SetSize(a.width, a.height)
			a.search.input.Focus()
			return a, a.search.input.Cursor.BlinkCmd()
		}
		// 'p' opens preview overlay
		if msg.String() == "p" {
			g := a.tree.findGroupForCursor()
			if g != nil {
				sid, _ := a.tree.findTargetForCursor()
				a.showPreview = true
				a.preview.SetGroup(g)
				a.preview.SetSession(sid)
				a.preview.SetSize(a.width, a.height)
				if !a.preview.loaded && sid != "" {
					return a, a.fetchConversation(sid)
				}
			}
			return a, nil
		}
	}

	// Delegate to tree model
	var cmd tea.Cmd
	a.tree, cmd = a.tree.Update(msg)
	a.tree.SaveExpandState(a.expandState)
	return a, cmd
}

func (a *App) View() string {
	if a.showHelp {
		return RenderHelpOverlay(a.tree.keys, a.width, a.height)
	}

	// Search overlay
	if a.showSearch {
		header := renderBanner(a.width, a.groups, a.filterText, a.err != nil, a.tick, a.steerKitAvailable)
		var footer string
		if len(a.search.results) == 0 && !a.search.querying {
			footer = a.search.input.View()
		} else {
			footer = HelpStyle.Render("  ↑↓ navigate  ⏎ select  / new search  esc back")
		}
		a.search.SetSize(a.width, a.height)
		return lipgloss.JoinVertical(lipgloss.Left, header, a.search.View(), footer)
	}

	// Preview overlay
	if a.showPreview {
		return a.preview.View()
	}

	// Header
	header := renderBanner(a.width, a.groups, a.filterText, a.err != nil, a.tick, a.steerKitAvailable)

	// Footer
	var footer string
	if a.filtering {
		footer = a.filter.View()
	} else if a.steerKitAvailable {
		footer = HelpStyle.Render("  ↑↓ navigate  ←→ expand  ⏎ open  n new  / filter  s search  p preview  ? help  q quit")
	} else {
		footer = HelpStyle.Render("  ↑↓ navigate  ←→ expand  ⏎ open  n new  / filter  p preview  ? help  q quit")
	}

	contentHeight := a.height - 2 // header + footer
	if contentHeight < 1 {
		contentHeight = 1
	}

	treeContent := a.tree.View()
	return lipgloss.JoinVertical(lipgloss.Left, header, treeContent, footer)
}

func renderPreviewBar(done, total, barLen int) string {
	if total == 0 {
		return ""
	}
	filled := done * barLen / total
	empty := barLen - filled
	return ProgressFull.Render(strings.Repeat("█", filled)) +
		ProgressEmpty.Render(strings.Repeat("░", empty))
}

func joinHelp(parts []string) []string {
	var result []string
	for i, p := range parts {
		if i > 0 {
			result = append(result, DimStyle.Render("  "))
		}
		result = append(result, HelpStyle.Render(p))
	}
	return result
}
