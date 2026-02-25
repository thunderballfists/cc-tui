package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var helpBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("4")).
	Padding(1, 2).
	Width(40)

// RenderHelpOverlay returns a centered help overlay string.
func RenderHelpOverlay(km KeyMap, width, height int) string {
	var lines []string

	lines = append(lines, HeaderStyle.Render("CC-TUI Keybindings"))
	lines = append(lines, "")

	sections := km.FullHelp()
	labels := []string{"Navigation", "Actions", "Other", "Jump"}

	for i, section := range sections {
		if i < len(labels) {
			lines = append(lines, TitleStyle.Render(labels[i]))
		}
		for _, binding := range section {
			help := binding.Help()
			lines = append(lines, "  "+lipgloss.NewStyle().Width(10).Render(help.Key)+" "+DimStyle.Render(help.Desc))
		}
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")
	box := helpBoxStyle.Render(content)

	// Center the box
	boxWidth := lipgloss.Width(box)
	boxHeight := lipgloss.Height(box)

	padX := (width - boxWidth) / 2
	padY := (height - boxHeight) / 2
	if padX < 0 {
		padX = 0
	}
	if padY < 0 {
		padY = 0
	}

	// Build the centered overlay
	var result strings.Builder
	for i := 0; i < padY; i++ {
		result.WriteString("\n")
	}
	for _, line := range strings.Split(box, "\n") {
		result.WriteString(strings.Repeat(" ", padX))
		result.WriteString(line)
		result.WriteString("\n")
	}

	return result.String()
}
