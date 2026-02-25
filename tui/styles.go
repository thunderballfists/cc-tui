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
