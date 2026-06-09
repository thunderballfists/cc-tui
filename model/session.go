package model

import "time"

// Session is a single CC conversation snapshot.
type Session struct {
	ID         string    `json:"id"`
	Project    string    `json:"project"`
	DirName    string    `json:"dir_name"`
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	LastActive time.Time `json:"last_active"`
	GitBranch  string    `json:"git_branch"`
	LastMsg    string    `json:"last_msg"`
	Summary    string    `json:"summary,omitempty"`        // session summary from SteerKit
	ContextTokens int    `json:"context_tokens,omitempty"` // live context size (tokens)

	Plan  *Plan  `json:"plan,omitempty"`
	Tasks []Task `json:"tasks,omitempty"`
	Todos []Todo `json:"todos,omitempty"`

	Active    bool   `json:"active"`
	PaneID    string `json:"pane_id"`
	PaneLabel string `json:"pane_label"`
}

// ProjectGroup groups multiple sessions (snapshots) for the same project.
// The first session is the most recent and provides plan/tasks/todos.
type ProjectGroup struct {
	Project    string    `json:"project"`
	DirName    string    `json:"dir_name"`
	Active     bool      `json:"active"`
	PaneID     string    `json:"pane_id"`
	PaneLabel  string    `json:"pane_label"`
	LastActive time.Time `json:"last_active"`
	Sessions   []Session `json:"sessions"` // sorted most recent first
}

// ArchivedSession is a session transcript preserved in the archive mirror.
type ArchivedSession struct {
	ID            string    `json:"id"`
	Project       string    `json:"project"`
	Title         string    `json:"title"`
	LastMsg       string    `json:"last_msg"`
	LastActive    time.Time `json:"last_active"`
	ContextTokens int       `json:"context_tokens,omitempty"`
	SizeBytes     int64     `json:"size_bytes"`
	LiveCopy      bool      `json:"live_copy"` // a resumable copy still exists in ~/.claude/projects
}

// ArchivedGroup groups archived sessions for one project.
type ArchivedGroup struct {
	Project  string            `json:"project"`
	DirName  string            `json:"dir_name"`
	Sessions []ArchivedSession `json:"sessions"` // sorted most recent first
}
