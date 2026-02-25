package tui

import (
	"encoding/json"
	"net"

	"cc-tui/model"
	"cc-tui/protocol"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type App struct {
	tree     TreeModel
	conn     net.Conn
	enc      *json.Encoder
	dec      *json.Decoder
	width    int
	height   int
	showHelp bool
	err      error
}

type sessionsMsg []model.Session
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

func NewApp(conn net.Conn) *App {
	return &App{
		tree: NewTreeModel(),
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(conn),
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.fetchTree(),
		tea.EnableMouseCellMotion,
	)
}

func (a *App) fetchTree() tea.Cmd {
	return func() tea.Msg {
		a.enc.Encode(protocol.Request{Cmd: "tree"})
		var resp protocol.Response
		if err := a.dec.Decode(&resp); err != nil {
			return errMsg{err}
		}
		return sessionsMsg(resp.Sessions)
	}
}

func (a *App) sendAction(action, sessionID, project string) tea.Cmd {
	return func() tea.Msg {
		a.enc.Encode(protocol.Request{
			Cmd:       "action",
			Action:    action,
			SessionID: project, // use project path as identifier
		})
		var resp protocol.Response
		if err := a.dec.Decode(&resp); err != nil {
			return errMsg{err}
		}
		if resp.Type == "error" {
			// Fallback: try with session ID
			a.enc.Encode(protocol.Request{
				Cmd:       "action",
				Action:    action,
				SessionID: sessionID,
			})
			a.dec.Decode(&resp)
		}
		return nil
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.tree.SetSize(msg.Width-2, msg.Height) // account for border padding
		return a, nil

	case sessionsMsg:
		a.tree.SetSessions([]model.Session(msg))
		return a, a.subscribeUpdates()

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

func (a *App) subscribeUpdates() tea.Cmd {
	return func() tea.Msg {
		a.enc.Encode(protocol.Request{Cmd: "subscribe"})
		var resp protocol.Response
		if err := a.dec.Decode(&resp); err != nil {
			return errMsg{err}
		}
		return sessionsMsg(resp.Sessions)
	}
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
