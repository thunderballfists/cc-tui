package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Session indicators
	ActiveDot   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff87")) // bright green
	InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))    // visible gray
	SessionName = lipgloss.NewStyle().Bold(true)
	ActiveName  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ff87"))

	// Content styles
	TitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaf00")) // warm yellow
	PlanLabel  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5f87ff")) // soft blue
	DimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	WIPStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaf00"))
	DoneStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff87"))

	// Navigation
	CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff87")).Bold(true)

	// Disclosure triangles — bold so they stand out
	ArrowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)

	// Layout
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#5f87ff")).
			Padding(0, 1)
	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5f87ff")).
			Bold(true)
	BannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1c1c1c")).
			Background(lipgloss.Color("#5f87ff")).
			Padding(0, 1)
	HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Progress bar
	ProgressFull  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff87"))
	ProgressEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))

	// Tree connectors
	TreeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))

	// Snapshot
	SnapshotStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	// Count/progress right-justified
	CountStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// Tree drawing characters
const (
	TreeVert   = "│"
	TreeBranch = "├─"
	TreeLast   = "└─"
	TreeBlank  = "  "

	// Larger, bolder triangles
	ArrowDown  = "▼"
	ArrowRight = "▶"

	// Larger dots
	DotActive   = "●"
	DotInactive = "○"

	CheckDone = "✓"
	CheckWIP  = "◐"
	CheckOpen = "○"
)
