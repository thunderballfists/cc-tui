package daemon

import (
	"testing"

	"cc-tui/model"
)

// Mirrors the runtime bug: an active project group whose latest session is
// running in a pane, plus several older inactive sessions. Selecting an older
// session must NOT inherit the group/latest session's Active flag and pane —
// otherwise "open" switches to the running latest session instead of resuming
// the selected one.
func activeGroupWithHistory() []model.ProjectGroup {
	return []model.ProjectGroup{
		{
			Project:   "/Users/me/dih-636",
			DirName:   "dih-636",
			Active:    true,
			PaneLabel: "main:1.1",
			Sessions: []model.Session{
				{ID: "latest-running", Project: "/Users/me/dih-636", Active: true, PaneLabel: "main:1.1"},
				{ID: "older-a", Project: "/Users/me/dih-636"},
				{ID: "older-b", Project: "/Users/me/dih-636"},
			},
		},
	}
}

func TestResolveActionTarget_NonLatestSessionInActiveGroup(t *testing.T) {
	groups := activeGroupWithHistory()

	got := resolveActionTarget(groups, "older-b")

	if !got.Found {
		t.Fatalf("expected to find session older-b")
	}
	if got.SessionID != "older-b" {
		t.Errorf("SessionID = %q, want older-b", got.SessionID)
	}
	// The bug: these came from the group, marking the older session as active
	// and pointing at the latest session's pane.
	if got.Active {
		t.Errorf("Active = true, want false (older session is not running)")
	}
	if got.PaneLabel != "" {
		t.Errorf("PaneLabel = %q, want empty (older session has no pane)", got.PaneLabel)
	}
}

func TestResolveActionTarget_LatestRunningSession(t *testing.T) {
	groups := activeGroupWithHistory()

	got := resolveActionTarget(groups, "latest-running")

	if !got.Found || got.SessionID != "latest-running" {
		t.Fatalf("expected to find latest-running, got %+v", got)
	}
	if !got.Active {
		t.Errorf("Active = false, want true (latest session is running)")
	}
	if got.PaneLabel != "main:1.1" {
		t.Errorf("PaneLabel = %q, want main:1.1", got.PaneLabel)
	}
}

func TestResolveActionTarget_ProjectPathFallback(t *testing.T) {
	groups := activeGroupWithHistory()

	got := resolveActionTarget(groups, "/Users/me/dih-636")

	if !got.Found {
		t.Fatalf("expected to find by project path")
	}
	// Project-path match uses the latest session.
	if got.SessionID != "latest-running" {
		t.Errorf("SessionID = %q, want latest-running", got.SessionID)
	}
	if !got.Active || got.PaneLabel != "main:1.1" {
		t.Errorf("expected group active/pane, got Active=%v PaneLabel=%q", got.Active, got.PaneLabel)
	}
}

func TestResolveActionTarget_NotFound(t *testing.T) {
	got := resolveActionTarget(activeGroupWithHistory(), "does-not-exist")
	if got.Found {
		t.Errorf("expected Found=false for unknown id, got %+v", got)
	}
}
