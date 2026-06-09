package tui

import (
	"testing"

	"cc-tui/model"
)

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{-1, "0 B"},
		{1536, "2 KB"}, // 1.5 KB rounds to 2
		{2048, "2 KB"},
		{5 * 1 << 20, "5.0 MB"},
		{int64(1.5 * float64(int64(1)<<30)), "1.50 GB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.n); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func sampleArchive() ([]model.ArchivedGroup, int64) {
	return []model.ArchivedGroup{
		{
			Project: "/p/a", DirName: "a",
			Sessions: []model.ArchivedSession{
				{ID: "11111111-1111-1111-1111-111111111111", Title: "first"},
				{ID: "22222222-2222-2222-2222-222222222222", Title: "second", LiveCopy: true},
			},
		},
		{
			Project: "/p/b", DirName: "b",
			Sessions: []model.ArchivedSession{
				{ID: "33333333-3333-3333-3333-333333333333", Title: "third"},
			},
		},
	}, 12345
}

func TestArchiveStateRowsAndSelection(t *testing.T) {
	var a ArchiveState
	a.SetSize(80, 40)
	groups, bytes := sampleArchive()
	a.SetData(groups, bytes)

	// 2 headers + 3 sessions = 5 rows
	if len(a.rows) != 5 {
		t.Fatalf("got %d rows, want 5", len(a.rows))
	}
	// Row 0 is a header → no selection
	a.cursor = 0
	if _, ok := a.selected(); ok {
		t.Error("header row should not be selectable")
	}
	// Row 1 is the first session
	a.cursor = 1
	s, ok := a.selected()
	if !ok || s.Title != "first" {
		t.Errorf("row 1 selected = %v %q, want true 'first'", ok, s.Title)
	}
}

func TestArchiveStateMoveCursorBounds(t *testing.T) {
	var a ArchiveState
	a.SetSize(80, 40)
	groups, bytes := sampleArchive()
	a.SetData(groups, bytes)

	a.moveCursor(-10)
	if a.cursor != 0 {
		t.Errorf("cursor = %d after moving up past top, want 0", a.cursor)
	}
	a.moveCursor(100)
	if a.cursor != len(a.rows)-1 {
		t.Errorf("cursor = %d after moving down past bottom, want %d", a.cursor, len(a.rows)-1)
	}
}

func TestArchiveStateMarkRestored(t *testing.T) {
	var a ArchiveState
	a.SetSize(80, 40)
	groups, bytes := sampleArchive()
	a.SetData(groups, bytes)

	id := "11111111-1111-1111-1111-111111111111"
	a.markRestored(id)
	for _, r := range a.rows {
		if !r.isHeader && r.session.ID == id && !r.session.LiveCopy {
			t.Error("expected LiveCopy=true after markRestored")
		}
	}
}
