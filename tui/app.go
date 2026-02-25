package tui

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cc-tui/model"
	"cc-tui/protocol"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type App struct {
	tree     TreeModel
	sockPath string
	mu       sync.Mutex
	width    int
	height   int
	showHelp bool
	err      error
}

type sessionsMsg []model.Session
type errMsg struct{ err error }
type tickMsg time.Time

func (e errMsg) Error() string { return e.err.Error() }

func NewApp(conn net.Conn) *App {
	// We close the initial probe connection; we'll connect fresh for each request
	conn.Close()

	home, _ := os.UserHomeDir()
	sockPath := filepath.Join(home, ".claude", "cc-tui.sock")

	return &App{
		tree:     NewTreeModel(),
		sockPath: sockPath,
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.fetchTree(),
		tea.EnableMouseCellMotion,
		tickCmd(),
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
		return sessionsMsg(resp.Sessions)
	}
}

func (a *App) sendAction(action, sessionID, project string) tea.Cmd {
	return func() tea.Msg {
		resp, err := a.doRequest(protocol.Request{
			Cmd:       "action",
			Action:    action,
			SessionID: project,
		})
		if err != nil {
			return errMsg{err}
		}
		if resp.Type == "error" {
			// Fallback: try with session ID
			a.doRequest(protocol.Request{
				Cmd:       "action",
				Action:    action,
				SessionID: sessionID,
			})
		}
		return nil
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.tree.SetSize(msg.Width-2, msg.Height)
		return a, nil

	case sessionsMsg:
		a.err = nil
		a.tree.SetSessions([]model.Session(msg))
		return a, nil

	case tickMsg:
		return a, tea.Batch(a.fetchTree(), tickCmd())

	case errMsg:
		a.err = msg.err
		return a, nil

	case ActionMsg:
		return a, a.sendAction(msg.Action, msg.SessionID, msg.Project)

	case RefreshMsg:
		return a, a.fetchTree()

	case tea.KeyMsg:
		// Global keys
		if a.showHelp {
			a.showHelp = false
			return a, nil
		}
		if key.Matches(msg, a.tree.keys.Help) {
			a.showHelp = !a.showHelp
			return a, nil
		}
		if key.Matches(msg, a.tree.keys.Quit) {
			return a, tea.Quit
		}
	}

	// Delegate to tree model
	var cmd tea.Cmd
	a.tree, cmd = a.tree.Update(msg)
	return a, cmd
}

func (a *App) View() string {
	if a.showHelp {
		return RenderHelpOverlay(a.tree.keys, a.width, a.height)
	}

	header := HeaderStyle.Render(" CC Sessions ")
	tree := a.tree.View()

	// Help footer
	helpKeys := a.tree.keys.ShortHelp()
	var helpParts []string
	for _, k := range helpKeys {
		h := k.Help()
		helpParts = append(helpParts, h.Key+" "+h.Desc)
	}
	help := HelpStyle.Render("  " + lipgloss.JoinHorizontal(lipgloss.Left, joinHelp(helpParts)...) + "  ? help")

	if a.err != nil {
		header += " " + lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(a.err.Error())
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, tree, help)
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
