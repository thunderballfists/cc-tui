package tui

import (
	"fmt"
	"strings"

	"cc-tui/model"

	"github.com/charmbracelet/lipgloss"
)

// Blue → purple gradient
var bannerGradient = []string{
	"#3a5fcd", "#4563d0", "#5067d3", "#5b6bd6", "#666fd9",
	"#7173dc", "#7c77df", "#877be2", "#927fe5", "#9d83e8",
}

// Divider pulse colors — high contrast against the purple end
var dividerColors = []string{
	"#00ffcc", // cyan/teal
	"#ffffff", // white
}

// renderBanner creates a full-width gradient banner with title and status.
func renderBanner(width int, groups []model.ProjectGroup, filterText string, hasErr bool, tick int) string {
	if width < 10 {
		width = 10
	}

	activeCount := 0
	totalSessions := 0
	for _, g := range groups {
		if g.Active {
			activeCount++
		}
		totalSessions += len(g.Sessions)
	}

	title := " ⚡CC Sessions "
	titleLen := lipgloss.Width(title)
	right := buildRightStatus(width-titleLen-2, activeCount, len(groups), totalSessions, filterText)

	totalLen := width
	rightLen := lipgloss.Width(right)
	midLen := totalLen - titleLen - rightLen
	if midLen < 0 {
		midLen = 0
		right = ""
		rightLen = 0
		midLen = totalLen - titleLen
		if midLen < 0 {
			midLen = 0
		}
	}

	fg := lipgloss.Color("#ffffff")
	fgRight := lipgloss.Color("#e0e0f0")
	fgAccent := lipgloss.Color("#00ff87")                                    // green ● dot
	fgDivider := lipgloss.Color(dividerColors[tick%len(dividerColors)]) // pulsing divider

	var result strings.Builder
	pos := 0 // visible column position

	// Helper to pick gradient bg for a column
	bgFor := func(col int) lipgloss.Color {
		idx := col * len(bannerGradient) / totalLen
		if idx >= len(bannerGradient) {
			idx = len(bannerGradient) - 1
		}
		if idx < 0 {
			idx = 0
		}
		return lipgloss.Color(bannerGradient[idx])
	}

	// Title section
	for _, ch := range title {
		style := lipgloss.NewStyle().
			Foreground(fg).
			Background(bgFor(pos)).
			Bold(true)
		result.WriteString(style.Render(string(ch)))
		pos++
	}

	// Middle fill
	for i := 0; i < midLen; i++ {
		result.WriteString(lipgloss.NewStyle().Background(bgFor(pos)).Render(" "))
		pos++
	}

	// Right section
	for _, ch := range right {
		charFg := fgRight
		switch ch {
		case '●':
			charFg = fgAccent
		case '◆':
			charFg = fgDivider
		}
		style := lipgloss.NewStyle().
			Foreground(charFg).
			Background(bgFor(pos))
		result.WriteString(style.Render(string(ch)))
		pos++
	}

	// Fill any remaining columns (rounding)
	for pos < totalLen {
		result.WriteString(lipgloss.NewStyle().Background(bgFor(pos)).Render(" "))
		pos++
	}

	return result.String()
}

func buildRightStatus(avail, active, projects, sessions int, filter string) string {
	if avail < 5 {
		return ""
	}

	candidates := []string{}
	div := "◆"

	if filter != "" {
		candidates = append(candidates,
			fmt.Sprintf(" 🔍%s ◆ ● %d ◆ %dp %ds ", filter, active, projects, sessions),
		)
	}

	if active > 0 {
		candidates = append(candidates,
			fmt.Sprintf(" ● %d active %s %d proj %s %d sess ", active, div, projects, div, sessions),
			fmt.Sprintf(" ● %d %s %dp %ds ", active, div, projects, sessions),
			fmt.Sprintf(" ● %d %s %dp ", active, div, projects),
			fmt.Sprintf(" ● %d ", active),
		)
	} else {
		candidates = append(candidates,
			fmt.Sprintf(" %d proj %s %d sess ", projects, div, sessions),
			fmt.Sprintf(" %dp %s %ds ", projects, div, sessions),
			fmt.Sprintf(" %dp ", projects),
		)
	}

	for _, c := range candidates {
		if lipgloss.Width(c) <= avail {
			return c
		}
	}
	return ""
}
