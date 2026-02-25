package protocol

import "cc-tui/model"

// Client -> Daemon requests
type Request struct {
	Cmd       string `json:"cmd"`                 // tree, subscribe, action
	Action    string `json:"action,omitempty"`     // open, window, new
	SessionID string `json:"session_id,omitempty"`
}

// Daemon -> Client responses
type Response struct {
	Type     string          `json:"type"`               // snapshot, update, error, ok
	Sessions []model.Session `json:"sessions,omitempty"`
	Error    string          `json:"error,omitempty"`
}
