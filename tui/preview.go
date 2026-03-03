package tui

import (
	"fmt"
	"strings"

	"cc-tui/model"

	"github.com/charmbracelet/lipgloss"
)

// PreviewState holds the state for the preview overlay.
type PreviewState struct {
	group     *model.ProjectGroup
	sessionID string
	messages  []model.ConvMessage
	loaded    bool
	scroll    int
	search    string
	searching bool
	dragging  bool
	width     int
	height    int
}

func (p *PreviewState) SetGroup(g *model.ProjectGroup) {
	if p.group == nil || g == nil || p.group.Project != g.Project {
		p.messages = nil
		p.loaded = false
		p.scroll = 0
		p.search = ""
	}
	p.group = g
}

func (p *PreviewState) SetSession(id string) {
	if p.sessionID != id {
		p.sessionID = id
		p.messages = nil
		p.loaded = false
		p.scroll = 0
	}
}

func (p *PreviewState) SetMessages(msgs []model.ConvMessage) {
	p.messages = msgs
	p.loaded = true
}

func (p *PreviewState) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *PreviewState) ScrollUp(n int) {
	p.scroll -= n
	if p.scroll < 0 {
		p.scroll = 0
	}
}

func (p *PreviewState) ScrollDown(n int) {
	maxScroll := len(p.buildLines()) - p.contentHeight()
	if maxScroll < 0 {
		maxScroll = 0
	}
	p.scroll += n
	if p.scroll > maxScroll {
		p.scroll = maxScroll
	}
}

// HandleMouse processes mouse events for scrollbar dragging and scroll wheel.
func (p *PreviewState) HandleMouse(y int, isWheel bool, wheelUp bool) {
	totalLines := len(p.buildLines())
	ch := p.contentHeight()
	if totalLines <= ch {
		return
	}

	if isWheel {
		if wheelUp {
			p.ScrollUp(3)
		} else {
			p.ScrollDown(3)
		}
		return
	}

	// Mouse click/drag on scrollbar — map Y to scroll position
	// Y is relative to the terminal. The box border starts at ~row 1, content at row 2.
	// Scrollbar occupies rows 2..2+ch within the box.
	barY := y - 2 // offset for border
	if barY < 0 {
		barY = 0
	}
	if barY >= ch {
		barY = ch - 1
	}

	// Map barY to scroll position
	maxScroll := totalLines - ch
	p.scroll = barY * maxScroll / ch
	if p.scroll < 0 {
		p.scroll = 0
	}
	if p.scroll > maxScroll {
		p.scroll = maxScroll
	}
}

func (p *PreviewState) contentHeight() int {
	h := p.height - 4 // border + footer
	if h < 1 {
		h = 1
	}
	return h
}

func (p *PreviewState) buildLines() []string {
	var lines []string
	g := p.group
	if g == nil {
		return []string{DimStyle.Render("No session selected")}
	}

	w := p.width - 6 // border + padding
	if w < 10 {
		w = 10
	}
	trunc := lipgloss.NewStyle().MaxWidth(w)

	// --- Header ---
	dot := "○"
	dotStyle := InactiveDot
	if g.Active {
		dot = "●"
		dotStyle = ActiveDot
	}
	lines = append(lines, dotStyle.Render(dot)+" "+SessionName.Render(g.DirName))

	if len(g.Sessions) > 0 {
		latest := &g.Sessions[0]
		if latest.Title != "" {
			lines = append(lines, trunc.Render(TitleStyle.Render(latest.Title)))
		}
		if latest.GitBranch != "" {
			lines = append(lines, trunc.Render(DimStyle.Render("branch: "+latest.GitBranch)))
		}
	}
	lines = append(lines, "")

	// --- Plan summary ---
	if len(g.Sessions) > 0 {
		latest := &g.Sessions[0]
		if latest.Plan != nil && len(latest.Plan.Steps) > 0 {
			title := latest.Plan.Title
			if strings.HasPrefix(title, "Plan: ") {
				title = title[6:]
			}
			done := 0
			for _, step := range latest.Plan.Steps {
				if step.Status == model.StepDone {
					done++
				}
			}
			total := len(latest.Plan.Steps)
			bar := renderPreviewBar(done, total, 10)
			lines = append(lines, trunc.Render(PlanLabel.Render("Plan: ")+title))
			lines = append(lines, fmt.Sprintf("%s %d/%d", bar, done, total))

			for _, step := range latest.Plan.Steps {
				var sl string
				switch step.Status {
				case model.StepDone:
					sl = DoneStyle.Render(CheckDone) + " " + DimStyle.Render(step.Text)
				case model.StepWIP:
					sl = WIPStyle.Render(CheckWIP) + " " + step.Text
				default:
					sl = DimStyle.Render(CheckOpen+" " + step.Text)
				}
				lines = append(lines, trunc.Render(sl))
			}
			lines = append(lines, "")
		}

		// Tasks
		if len(latest.Tasks) > 0 {
			done := 0
			for _, t := range latest.Tasks {
				if t.Status == "completed" {
					done++
				}
			}
			bar := renderPreviewBar(done, len(latest.Tasks), 10)
			lines = append(lines, fmt.Sprintf("Tasks  %s %d/%d", bar, done, len(latest.Tasks)))
			for _, t := range latest.Tasks {
				subj := t.Subject
				if subj == "" {
					subj = t.ActiveForm
				}
				var tl string
				switch t.Status {
				case "completed":
					tl = DoneStyle.Render(CheckDone) + " " + DimStyle.Render(subj)
				case "in_progress":
					tl = WIPStyle.Render(CheckWIP) + " " + subj
				default:
					tl = DimStyle.Render(CheckOpen+" " + subj)
				}
				lines = append(lines, trunc.Render(tl))
			}
			lines = append(lines, "")
		}

		// Todos
		if len(latest.Todos) > 0 {
			done := 0
			for _, t := range latest.Todos {
				if t.Status == "completed" {
					done++
				}
			}
			bar := renderPreviewBar(done, len(latest.Todos), 10)
			lines = append(lines, fmt.Sprintf("Todos  %s %d/%d", bar, done, len(latest.Todos)))
			for _, t := range latest.Todos {
				content := t.ActiveForm
				if content == "" {
					content = t.Content
				}
				var tl string
				switch t.Status {
				case "completed":
					tl = DoneStyle.Render(CheckDone) + " " + DimStyle.Render(content)
				case "in_progress":
					tl = WIPStyle.Render(CheckWIP) + " " + content
				default:
					tl = DimStyle.Render(CheckOpen+" " + content)
				}
				lines = append(lines, trunc.Render(tl))
			}
			lines = append(lines, "")
		}
	}

	// --- Conversation ---
	if p.loaded && len(p.messages) > 0 {
		lines = append(lines, strings.Repeat("─", min(w, 40)))
		lines = append(lines, HeaderStyle.Render("Conversation")+"  "+DimStyle.Render(fmt.Sprintf("(%d messages)", len(p.messages))))
		lines = append(lines, "")

		searchLower := strings.ToLower(p.search)

		for _, msg := range p.messages {
			// Filter by search
			if searchLower != "" {
				if !strings.Contains(strings.ToLower(msg.Content), searchLower) {
					continue
				}
			}

			var roleStyle lipgloss.Style
			var prefix string
			if msg.Role == "user" {
				roleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
				prefix = "▸ you"
			} else {
				roleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
				prefix = "◂ claude"
			}
			if msg.Time != "" {
				prefix += " " + DimStyle.Render(msg.Time)
			}
			lines = append(lines, roleStyle.Render(prefix))

			// Wrap content to width
			content := msg.Content
			if searchLower != "" {
				content = highlightSearch(content, p.search)
			}
			for _, cl := range wrapText(content, w) {
				lines = append(lines, trunc.Render(cl))
			}
			lines = append(lines, "")
		}
	} else if !p.loaded {
		lines = append(lines, DimStyle.Render("Loading conversation..."))
	}

	return lines
}

func (p *PreviewState) View() string {
	lines := p.buildLines()

	ch := p.contentHeight()
	totalLines := len(lines)

	// Apply scroll
	start := p.scroll
	if start > totalLines {
		start = totalLines
	}
	end := start + ch
	if end > totalLines {
		end = totalLines
	}
	visible := lines[start:end]

	// Build scrollbar track
	scrollbar := p.buildScrollbar(ch, totalLines)

	// Combine content lines with scrollbar on the right
	contentWidth := p.width - 5 // border(2) + padding(2) + scrollbar(1)
	if contentWidth < 10 {
		contentWidth = 10
	}
	trunc := lipgloss.NewStyle().MaxWidth(contentWidth)

	var contentLines []string
	for i, line := range visible {
		truncLine := trunc.Render(line)
		// Pad to width, append scrollbar char
		padded := truncLine
		visW := lipgloss.Width(padded)
		if visW < contentWidth {
			padded += strings.Repeat(" ", contentWidth-visW)
		}
		sb := " "
		if i < len(scrollbar) {
			sb = scrollbar[i]
		}
		contentLines = append(contentLines, padded+sb)
	}
	// Pad remaining viewport height
	for i := len(contentLines); i < ch; i++ {
		sb := " "
		if i < len(scrollbar) {
			sb = scrollbar[i]
		}
		contentLines = append(contentLines, strings.Repeat(" ", contentWidth)+sb)
	}

	content := strings.Join(contentLines, "\n")

	// Scroll percentage
	scrollInfo := ""
	if totalLines > ch {
		pct := 0
		if totalLines-ch > 0 {
			pct = p.scroll * 100 / (totalLines - ch)
		}
		scrollInfo = DimStyle.Render(fmt.Sprintf(" %d%%", pct))
	}

	// Footer
	var footer string
	if p.searching {
		footer = "/ " + p.search + "█"
	} else {
		footer = HelpStyle.Render("p close  jk/du scroll  / search  ⏎ open  q quit") + scrollInfo
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5f87ff")).
		Padding(0, 1).
		Width(p.width - 2).
		Height(ch).
		Render(content)

	return box + "\n" + footer
}

// buildScrollbar returns a column of characters representing the scroll position.
func (p *PreviewState) buildScrollbar(viewHeight, totalLines int) []string {
	bar := make([]string, viewHeight)

	if totalLines <= viewHeight {
		// No scrolling needed — no track
		for i := range bar {
			bar[i] = " "
		}
		return bar
	}

	// Calculate thumb position and size
	thumbSize := viewHeight * viewHeight / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	maxScroll := totalLines - viewHeight
	thumbPos := 0
	if maxScroll > 0 {
		thumbPos = p.scroll * (viewHeight - thumbSize) / maxScroll
	}

	trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#5f87ff"))

	for i := range bar {
		if i >= thumbPos && i < thumbPos+thumbSize {
			bar[i] = thumbStyle.Render("┃")
		} else {
			bar[i] = trackStyle.Render("│")
		}
	}
	return bar
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		if len(para) == 0 {
			lines = append(lines, "")
			continue
		}
		for len(para) > width {
			// Find last space before width
			cut := width
			for i := width; i > width/2; i-- {
				if para[i] == ' ' {
					cut = i
					break
				}
			}
			lines = append(lines, para[:cut])
			para = para[cut:]
			if len(para) > 0 && para[0] == ' ' {
				para = para[1:]
			}
		}
		if len(para) > 0 {
			lines = append(lines, para)
		}
	}
	return lines
}

func highlightSearch(text, query string) string {
	if query == "" {
		return text
	}
	lower := strings.ToLower(text)
	queryLower := strings.ToLower(query)
	hl := lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))

	var result strings.Builder
	pos := 0
	for {
		idx := strings.Index(lower[pos:], queryLower)
		if idx < 0 {
			result.WriteString(text[pos:])
			break
		}
		result.WriteString(text[pos : pos+idx])
		result.WriteString(hl.Render(text[pos+idx : pos+idx+len(query)]))
		pos = pos + idx + len(query)
	}
	return result.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
