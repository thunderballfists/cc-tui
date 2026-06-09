package tui

import (
	"fmt"
	"strings"

	"cc-tui/model"

	"github.com/charmbracelet/lipgloss"
)

// archiveRow is one flattened, navigable line in the archive view:
// either a project header or a session under it.
type archiveRow struct {
	isHeader bool
	dirName  string
	session  model.ArchivedSession
}

// ArchiveState manages the archive browser overlay.
type ArchiveState struct {
	groups     []model.ArchivedGroup
	rows       []archiveRow
	totalBytes int64
	cursor     int
	offset     int
	loaded     bool
	width      int
	height     int
}

func (a *ArchiveState) SetSize(w, h int) {
	a.width = w
	a.height = h
}

func (a *ArchiveState) Reset() {
	a.cursor = 0
	a.offset = 0
}

// SetData stores fetched archive data and rebuilds the flattened row list.
func (a *ArchiveState) SetData(groups []model.ArchivedGroup, totalBytes int64) {
	a.groups = groups
	a.totalBytes = totalBytes
	a.loaded = true
	a.rows = a.rows[:0]
	for _, g := range groups {
		a.rows = append(a.rows, archiveRow{isHeader: true, dirName: g.DirName})
		for _, s := range g.Sessions {
			a.rows = append(a.rows, archiveRow{session: s})
		}
	}
	if a.cursor >= len(a.rows) {
		a.cursor = len(a.rows) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

func (a *ArchiveState) contentHeight() int {
	h := a.height - 3 // banner + header line + footer
	if h < 1 {
		h = 1
	}
	return h
}

// selected returns the session under the cursor, or false if on a header/empty.
func (a *ArchiveState) selected() (model.ArchivedSession, bool) {
	if a.cursor < 0 || a.cursor >= len(a.rows) {
		return model.ArchivedSession{}, false
	}
	r := a.rows[a.cursor]
	if r.isHeader {
		return model.ArchivedSession{}, false
	}
	return r.session, true
}

func (a *ArchiveState) moveCursor(delta int) {
	n := len(a.rows)
	if n == 0 {
		return
	}
	a.cursor += delta
	if a.cursor < 0 {
		a.cursor = 0
	}
	if a.cursor >= n {
		a.cursor = n - 1
	}
	viewH := a.contentHeight()
	if a.cursor < a.offset {
		a.offset = a.cursor
	}
	if a.cursor >= a.offset+viewH {
		a.offset = a.cursor - viewH + 1
	}
}

// markRestored flips the live-copy flag on the session with the given ID.
func (a *ArchiveState) markRestored(id string) {
	for i := range a.rows {
		if !a.rows[i].isHeader && a.rows[i].session.ID == id {
			a.rows[i].session.LiveCopy = true
		}
	}
}

func (a *ArchiveState) View() string {
	if !a.loaded {
		return DimStyle.Render("  Loading archive…")
	}
	if len(a.rows) == 0 {
		return DimStyle.Render("  No archived sessions yet — sync runs daily.")
	}

	viewH := a.contentHeight()
	end := a.offset + viewH
	if end > len(a.rows) {
		end = len(a.rows)
	}

	var lines []string
	for i := a.offset; i < end; i++ {
		r := a.rows[i]
		selected := i == a.cursor

		var line string
		if r.isHeader {
			line = "  " + ArrowStyle.Render(ArrowDown) + " " + SessionName.Render(r.dirName)
		} else {
			s := r.session
			label := s.Title
			if label == "" {
				label = s.LastMsg
			}
			if label == "" {
				label = s.ID[:8]
			}
			maxLabel := a.width - 32
			if maxLabel < 15 {
				maxLabel = 15
			}
			if len(label) > maxLabel {
				label = label[:maxLabel] + "…"
			}

			size := ""
			if tok := formatTokens(s.ContextTokens); tok != "" {
				size = " " + TokenStyle.Render("• "+tok)
			}
			age := SnapshotStyle.Render(relativeTime(s.LastActive))
			live := ""
			if s.LiveCopy {
				live = " " + DoneStyle.Render("✓live")
			}
			prefix := "   " + TreeStyle.Render(TreeVert) + " "
			line = prefix + TitleStyle.Render(label) + size + " " + DimStyle.Render("│") + " " + age + live
		}

		if selected {
			line = CursorStyle.Render("▶") + line[2:]
		}
		if a.width > 0 {
			line = lipgloss.NewStyle().MaxWidth(a.width).Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// HeaderLine renders the disk-usage summary shown above the list.
func (a *ArchiveState) HeaderLine() string {
	count := 0
	for _, r := range a.rows {
		if !r.isHeader {
			count++
		}
	}
	return HeaderStyle.Render("Archive: ") +
		DimStyle.Render(fmt.Sprintf("%s across %d sessions", humanBytes(a.totalBytes), count))
}

// humanBytes formats a byte count as KB/MB/GB.
func humanBytes(n int64) string {
	switch {
	case n <= 0:
		return "0 B"
	case n < 1<<20:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	case n < 1<<30:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	}
}
