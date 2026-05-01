package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// SearchResult is a single result from SteerKit /recall.
type SearchResult struct {
	SourceType     string    // "exchange" or "episode"
	SessionID      string
	Score          float64
	Summary        string // exchange-level match summary
	SessionSummary string // session-level summary from /sessions
	Project        string
	Timestamp      time.Time
}

// SearchState manages the deep search overlay.
type SearchState struct {
	input     textinput.Model
	results   []SearchResult
	cursor    int
	offset    int
	querying  bool
	noResults bool
	width     int
	height    int
}

// NewSearchState returns an initialized SearchState.
func NewSearchState() SearchState {
	ti := textinput.New()
	ti.Prompt = "search: "
	ti.CharLimit = 80
	return SearchState{input: ti}
}

func (s *SearchState) SetSize(w, h int) {
	s.width = w
	s.height = h
}

func (s *SearchState) Reset() {
	s.input.Blur()
	s.input.SetValue("")
	s.results = nil
	s.cursor = 0
	s.offset = 0
	s.querying = false
	s.noResults = false
}

// ClearResults drops results but keeps the input, returning to query-input mode.
func (s *SearchState) ClearResults() {
	s.results = nil
	s.cursor = 0
	s.offset = 0
	s.noResults = false
}

func (s *SearchState) contentHeight() int {
	h := s.height - 3
	if h < 1 {
		h = 1
	}
	return h
}

// View renders the search results list.
func (s *SearchState) View() string {
	if s.querying {
		return DimStyle.Render("  Searching...")
	}
	if s.noResults {
		return DimStyle.Render(fmt.Sprintf("  No results for '%s'", s.input.Value()))
	}
	if len(s.results) == 0 {
		return DimStyle.Render("  Type a query and press Enter")
	}

	viewHeight := s.contentHeight()
	var lines []string

	end := s.offset + viewHeight
	if end > len(s.results) {
		end = len(s.results)
	}

	for i := s.offset; i < end; i++ {
		r := s.results[i]
		selected := i == s.cursor

		// Type indicator
		typeInd := "C"
		if r.SourceType == "episode" {
			typeInd = "E"
		}

		// Score
		score := DimStyle.Render(fmt.Sprintf("%.2f", r.Score))

		// Summary — prefer session summary for context, fall back to match summary
		summary := r.SessionSummary
		if summary == "" {
			summary = r.Summary
		}
		maxSummary := s.width - 30
		if maxSummary < 20 {
			maxSummary = 20
		}
		if len(summary) > maxSummary {
			summary = summary[:maxSummary] + "…"
		}

		// Project
		project := DimStyle.Render(r.Project)

		// Time
		ts := relativeTime(r.Timestamp)

		line := fmt.Sprintf(" %s  %s  %s  %s  %s",
			score,
			PlanLabel.Render(typeInd),
			summary,
			project,
			DimStyle.Render(ts),
		)

		if selected {
			line = CursorStyle.Render("▶") + line
		} else {
			line = " " + line
		}

		if s.width > 0 {
			line = lipgloss.NewStyle().MaxWidth(s.width).Render(line)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
