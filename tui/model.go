package tui

import (
	"strings"

	"cc-tui/model"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ActionMsg is sent when a session action should be dispatched.
type ActionMsg struct {
	Action    string // open, window, new
	SessionID string
	Project   string
}

// RefreshMsg requests data refresh from daemon.
type RefreshMsg struct{}

// TreeModel manages the tree state and keyboard navigation.
type TreeModel struct {
	roots        []*TreeNode
	visible      []*TreeNode
	cursor       int
	scrollOffset int
	height       int
	width        int
	keys         KeyMap
}

func NewTreeModel() TreeModel {
	return TreeModel{
		keys: DefaultKeyMap(),
	}
}

func (m *TreeModel) SetSessions(sessions []model.Session) {
	m.roots = BuildTree(sessions)
	m.rebuildVisible()
	// Clamp cursor
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *TreeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *TreeModel) rebuildVisible() {
	m.visible = FlattenVisible(m.roots)
}

func (m TreeModel) Update(msg tea.Msg) (TreeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
			m.ensureVisible()

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
			m.ensureVisible()

		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
			m.scrollOffset = 0

		case key.Matches(msg, m.keys.Bottom):
			m.cursor = len(m.visible) - 1
			m.ensureVisible()

		case key.Matches(msg, m.keys.Right):
			if m.cursor < len(m.visible) {
				node := m.visible[m.cursor]
				if len(node.Children) > 0 && !node.Expanded {
					node.Expanded = true
					m.rebuildVisible()
				} else if m.cursor < len(m.visible)-1 {
					// Move to first child
					m.cursor++
					m.ensureVisible()
				}
			}

		case key.Matches(msg, m.keys.Left):
			if m.cursor < len(m.visible) {
				node := m.visible[m.cursor]
				if node.Expanded && len(node.Children) > 0 {
					node.Expanded = false
					m.rebuildVisible()
				} else if node.Kind != NodeSession {
					// Jump to parent
					m.jumpToParent()
				}
			}

		case key.Matches(msg, m.keys.Open):
			return m, m.openAction("open")

		case key.Matches(msg, m.keys.Window):
			return m, m.openAction("window")

		case key.Matches(msg, m.keys.New):
			return m, m.openAction("new")

		case key.Matches(msg, m.keys.Refresh):
			return m, func() tea.Msg { return RefreshMsg{} }
		}
	}

	return m, nil
}

func (m *TreeModel) ensureVisible() {
	viewHeight := m.height - 3 // header + footer
	if viewHeight < 1 {
		viewHeight = 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+viewHeight {
		m.scrollOffset = m.cursor - viewHeight + 1
	}
}

func (m *TreeModel) jumpToParent() {
	if m.cursor >= len(m.visible) {
		return
	}
	node := m.visible[m.cursor]
	targetDepth := node.Depth - 1
	for i := m.cursor - 1; i >= 0; i-- {
		if m.visible[i].Depth <= targetDepth {
			m.cursor = i
			m.ensureVisible()
			return
		}
	}
}

func (m *TreeModel) findSessionForCursor() *model.Session {
	if m.cursor >= len(m.visible) {
		return nil
	}
	node := m.visible[m.cursor]

	// If it's a session node, use it directly
	if node.Kind == NodeSession && node.Session != nil {
		return node.Session
	}

	// Walk up to find parent session
	for i := m.cursor - 1; i >= 0; i-- {
		if m.visible[i].Kind == NodeSession && m.visible[i].Session != nil {
			return m.visible[i].Session
		}
	}
	return nil
}

func (m *TreeModel) openAction(action string) tea.Cmd {
	s := m.findSessionForCursor()
	if s == nil {
		return nil
	}
	return func() tea.Msg {
		return ActionMsg{
			Action:    action,
			SessionID: s.ID,
			Project:   s.Project,
		}
	}
}

func (m TreeModel) View() string {
	if len(m.visible) == 0 {
		return DimStyle.Render("  No sessions found")
	}

	viewHeight := m.height - 3
	if viewHeight < 1 {
		viewHeight = 1
	}

	var lines []string
	end := m.scrollOffset + viewHeight
	if end > len(m.visible) {
		end = len(m.visible)
	}

	for i := m.scrollOffset; i < end; i++ {
		lines = append(lines, RenderNode(m.visible[i], i, m.cursor, m.width))
	}

	return strings.Join(lines, "\n")
}
