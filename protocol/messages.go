package protocol

import "cc-tui/model"

// Client -> Daemon requests
type Request struct {
	Cmd       string `json:"cmd"`                  // tree, subscribe, action, conversation, archive-list, archive-restore, archive-open
	Action    string `json:"action,omitempty"`     // open, window, new
	SessionID string `json:"session_id,omitempty"` // session or project ID
}

// Daemon -> Client responses
type Response struct {
	Type         string                `json:"type"` // snapshot, update, error, ok, conversation, archives
	Groups       []model.ProjectGroup  `json:"groups,omitempty"`
	Conversation []model.ConvMessage   `json:"conversation,omitempty"`
	Archives     []model.ArchivedGroup `json:"archives,omitempty"`
	ArchiveBytes int64                 `json:"archive_bytes,omitempty"`
	Error        string                `json:"error,omitempty"`
}
