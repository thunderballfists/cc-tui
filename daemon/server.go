package daemon

import (
	"encoding/json"
	"net"
	"os"
	"time"

	"cc-tui/model"
	"cc-tui/protocol"
)

type Server struct {
	cache    *Cache
	sockPath string
	listener net.Listener
}

func NewServer(cache *Cache, sockPath string) *Server {
	return &Server{cache: cache, sockPath: sockPath}
}

func (s *Server) Start() error {
	os.Remove(s.sockPath)

	ln, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return err
	}
	s.listener = ln

	go s.acceptLoop()
	go s.pollActivePanes()

	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	for {
		var req protocol.Request
		if err := dec.Decode(&req); err != nil {
			return
		}

		switch req.Cmd {
		case "tree":
			groups := s.cache.Groups()
			enc.Encode(protocol.Response{
				Type:   "snapshot",
				Groups: groups,
			})

		case "action":
			s.handleAction(req, enc)

		case "conversation":
			s.handleConversation(req, enc)

		case "subscribe":
			s.streamUpdates(conn, enc)
			return
		}
	}
}

// actionTarget is the resolved session/project an action will operate on.
type actionTarget struct {
	SessionID string
	Project   string
	Active    bool
	PaneLabel string
	Found     bool
}

// resolveActionTarget finds the session/project an action refers to.
// reqID may be a session UUID (specific snapshot) or a project path (latest session).
// When matching a specific session, the Active/PaneLabel come from that session,
// not the group — selecting a non-latest snapshot must not inherit the latest
// session's running pane.
func resolveActionTarget(groups []model.ProjectGroup, reqID string) actionTarget {
	for _, g := range groups {
		// Match by session ID first (specific snapshot)
		for _, sess := range g.Sessions {
			if sess.ID == reqID {
				return actionTarget{
					SessionID: sess.ID,
					Project:   g.Project,
					Active:    sess.Active,
					PaneLabel: sess.PaneLabel,
					Found:     true,
				}
			}
		}
		// Fallback: match by project path (uses latest session)
		if g.Project == reqID {
			t := actionTarget{
				Project:   g.Project,
				Active:    g.Active,
				PaneLabel: g.PaneLabel,
				Found:     true,
			}
			if len(g.Sessions) > 0 {
				t.SessionID = g.Sessions[0].ID
			}
			return t
		}
	}
	return actionTarget{}
}

func (s *Server) handleAction(req protocol.Request, enc *json.Encoder) {
	t := resolveActionTarget(s.cache.Groups(), req.SessionID)

	if !t.Found {
		enc.Encode(protocol.Response{Type: "error", Error: "session not found"})
		return
	}

	// Use a login shell so claude is found via user's PATH (nvm, etc.)
	switch req.Action {
	case "open":
		if t.Active {
			TmuxSwitchToPane(t.PaneLabel)
		} else if t.SessionID != "" {
			TmuxSplitShell(t.Project, "claude --dangerously-skip-permissions -r "+t.SessionID)
		} else {
			TmuxSplitShell(t.Project, "claude --dangerously-skip-permissions")
		}
	case "window":
		if t.SessionID != "" {
			TmuxNewWindowShell(t.Project, "claude --dangerously-skip-permissions -r "+t.SessionID)
		} else {
			TmuxNewWindowShell(t.Project, "claude --dangerously-skip-permissions")
		}
	case "new":
		TmuxSplitShell(t.Project, "claude --dangerously-skip-permissions")
	}

	enc.Encode(protocol.Response{Type: "ok"})
}

func (s *Server) handleConversation(req protocol.Request, enc *json.Encoder) {
	groups := s.cache.Groups()
	dirs := s.cache.dirs

	// Find the session's JSONL path
	var jsonlPath string
	for _, g := range groups {
		// Match by session ID
		for _, sess := range g.Sessions {
			if sess.ID == req.SessionID {
				encoded := EncodeProjectPath(g.Project)
				jsonlPath = dirs.Projects + "/" + encoded + "/" + sess.ID + ".jsonl"
				break
			}
		}
		// Match by project path (use latest session)
		if jsonlPath == "" && g.Project == req.SessionID && len(g.Sessions) > 0 {
			encoded := EncodeProjectPath(g.Project)
			jsonlPath = dirs.Projects + "/" + encoded + "/" + g.Sessions[0].ID + ".jsonl"
		}
		if jsonlPath != "" {
			break
		}
	}

	if jsonlPath == "" {
		enc.Encode(protocol.Response{Type: "error", Error: "session not found"})
		return
	}

	msgs := LoadConversation(jsonlPath)
	enc.Encode(protocol.Response{
		Type:         "conversation",
		Conversation: msgs,
	})
}

func (s *Server) pollActivePanes() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		panes := GetActivePanes()
		s.cache.UpdateActiveStatus(panes)
	}
}

func (s *Server) streamUpdates(conn net.Conn, enc *json.Encoder) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		groups := s.cache.Groups()
		if err := enc.Encode(protocol.Response{
			Type:   "update",
			Groups: groups,
		}); err != nil {
			return
		}
	}
}

func (s *Server) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.sockPath)
}
