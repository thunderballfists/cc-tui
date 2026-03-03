package tui

import (
	"strings"
	"time"

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

const headerHeight = 1 // header line offset for mouse click mapping

// TreeModel manages the tree state and keyboard navigation.
type TreeModel struct {
	roots        []*TreeNode
	visible      []*TreeNode
	cursor       int
	scrollOffset int
	height       int
	width        int
	keys         KeyMap
	lastClickIdx int
	lastClickAt  time.Time
}

func NewTreeModel() TreeModel {
	return TreeModel{
		keys: DefaultKeyMap(),
	}
}

func (m *TreeModel) SetGroups(groups []model.ProjectGroup, expandState map[string]bool) {
	m.roots = BuildTree(groups)

	// Apply persisted expand state (overrides BuildTree defaults)
	for _, root := range m.roots {
		if root.Kind == NodeProject && root.Group != nil {
			key := "p:" + root.Group.Project
			if expanded, ok := expandState[key]; ok {
				root.Expanded = expanded
			}
			for _, child := range root.Children {
				ckey := key + "/" + child.Label
				if expanded, ok := expandState[ckey]; ok {
					child.Expanded = expanded
				}
			}
		}
	}

	m.rebuildVisible()
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// SaveExpandState captures current expand state into the provided map.
func (m *TreeModel) SaveExpandState(state map[string]bool) {
	for _, root := range m.roots {
		if root.Kind == NodeProject && root.Group != nil {
			key := "p:" + root.Group.Project
			state[key] = root.Expanded
			for _, child := range root.Children {
				ckey := key + "/" + child.Label
				state[ckey] = child.Expanded
			}
		}
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
	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			clickedIdx := msg.Y - headerHeight + m.scrollOffset
			if clickedIdx >= 0 && clickedIdx < len(m.visible) {
				now := time.Now()
				// Double-click detection: same node within 400ms
				if clickedIdx == m.lastClickIdx && now.Sub(m.lastClickAt) < 400*time.Millisecond {
					m.cursor = clickedIdx
					m.lastClickIdx = -1
					return m, m.openAction("open")
				}
				m.cursor = clickedIdx
				m.lastClickIdx = clickedIdx
				m.lastClickAt = now
				// Single click: toggle expand/collapse
				node := m.visible[clickedIdx]
				if len(node.Children) > 0 {
					node.Expanded = !node.Expanded
					m.rebuildVisible()
				}
			}
		} else if msg.Type == tea.MouseWheelUp {
			if m.scrollOffset > 0 {
				m.scrollOffset--
				if m.cursor > m.scrollOffset+m.height-4 {
					m.cursor = m.scrollOffset + m.height - 4
				}
			}
		} else if msg.Type == tea.MouseWheelDown {
			maxOffset := len(m.visible) - (m.height - 3)
			if maxOffset < 0 {
				maxOffset = 0
			}
			if m.scrollOffset < maxOffset {
				m.scrollOffset++
				if m.cursor < m.scrollOffset {
					m.cursor = m.scrollOffset
				}
			}
		}

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
				} else if node.Kind != NodeProject {
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

func (m *TreeModel) findGroupForCursor() *model.ProjectGroup {
	if m.cursor >= len(m.visible) {
		return nil
	}
	node := m.visible[m.cursor]
	if node.Kind == NodeProject && node.Group != nil {
		return node.Group
	}
	// Walk up to find parent project
	for i := m.cursor - 1; i >= 0; i-- {
		if m.visible[i].Kind == NodeProject && m.visible[i].Group != nil {
			return m.visible[i].Group
		}
	}
	return nil
}

func (m *TreeModel) findTargetForCursor() (sessionID, project string) {
	if m.cursor >= len(m.visible) {
		return "", ""
	}
	node := m.visible[m.cursor]

	// If it's a snapshot, use that specific session
	if node.Kind == NodeSnapshot && node.Session != nil {
		return node.Session.ID, node.Session.Project
	}

	// If it's a project node, use the latest session
	if node.Kind == NodeProject && node.Group != nil {
		g := node.Group
		if len(g.Sessions) > 0 {
			return g.Sessions[0].ID, g.Project
		}
		return "", g.Project
	}

	// Walk up to find parent project
	for i := m.cursor - 1; i >= 0; i-- {
		if m.visible[i].Kind == NodeProject && m.visible[i].Group != nil {
			g := m.visible[i].Group
			if len(g.Sessions) > 0 {
				return g.Sessions[0].ID, g.Project
			}
			return "", g.Project
		}
	}
	return "", ""
}

func (m *TreeModel) openAction(action string) tea.Cmd {
	sessionID, project := m.findTargetForCursor()
	if project == "" {
		return nil
	}
	return func() tea.Msg {
		return ActionMsg{
			Action:    action,
			SessionID: sessionID,
			Project:   project,
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
