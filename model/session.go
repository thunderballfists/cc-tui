package model

import "time"

type Session struct {
	ID         string    `json:"id"`
	Project    string    `json:"project"`
	DirName    string    `json:"dir_name"`
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	LastActive time.Time `json:"last_active"`
	GitBranch  string    `json:"git_branch"`
	LastMsg    string    `json:"last_msg"`

	Plan  *Plan  `json:"plan,omitempty"`
	Tasks []Task `json:"tasks,omitempty"`
	Todos []Todo `json:"todos,omitempty"`

	Active    bool   `json:"active"`
	PaneID    string `json:"pane_id"`
	PaneLabel string `json:"pane_label"`
}
