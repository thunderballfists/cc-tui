package daemon

import (
	"encoding/json"
	"net"
	"os"
	"time"

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
	// Clean up stale socket
	os.Remove(s.sockPath)

	ln, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return err
	}
	s.listener = ln

	go s.acceptLoop()

	// Periodic active pane polling
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
			sessions := s.cache.Sessions()
			enc.Encode(protocol.Response{
				Type:     "snapshot",
				Sessions: sessions,
			})

		case "action":
			s.handleAction(req, enc)

		case "subscribe":
			// Hold connection open, send updates when cache changes
			s.streamUpdates(conn, enc)
			return
		}
	}
}

func (s *Server) handleAction(req protocol.Request, enc *json.Encoder) {
	sessions := s.cache.Sessions()
	var target *Session
	for i := range sessions {
		if sessions[i].ID == req.SessionID || sessions[i].Project == req.SessionID {
			target = &Session{
				ID:        sessions[i].ID,
				Project:   sessions[i].Project,
				Active:    sessions[i].Active,
				PaneLabel: sessions[i].PaneLabel,
			}
			break
		}
	}
	if target == nil {
		enc.Encode(protocol.Response{Type: "error", Error: "session not found"})
		return
	}

	dir := target.Project
	claudeCmd := "claude --dangerously-skip-permissions"

	switch req.Action {
	case "open":
		if target.Active {
			TmuxSwitchToPane(target.PaneLabel)
		} else {
			TmuxSplitPane(dir, claudeCmd+" -r '"+target.ID+"'")
		}
	case "window":
		TmuxNewWindow(dir, claudeCmd+" -r '"+target.ID+"'")
	case "new":
		TmuxSplitPane(dir, claudeCmd)
	}

	enc.Encode(protocol.Response{Type: "ok"})
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
	// Simple polling approach: send snapshot every 2s
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		sessions := s.cache.Sessions()
		if err := enc.Encode(protocol.Response{
			Type:     "update",
			Sessions: sessions,
		}); err != nil {
			return // client disconnected
		}
	}
}

func (s *Server) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.sockPath)
}

// Session is a helper for action handling
type Session struct {
	ID        string
	Project   string
	Active    bool
	PaneLabel string
}
