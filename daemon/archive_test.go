package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// buildMirror creates a temp Dirs with an archive mirror containing one
// transcript, and a projects dir that may or may not have a live copy.
func buildMirror(t *testing.T, withLiveCopy bool) (Dirs, string) {
	t.Helper()
	root := t.TempDir()
	dirs := Dirs{
		Projects: filepath.Join(root, "projects"),
		Archive:  filepath.Join(root, "cc-tui-archive"),
	}
	uuid := "7022e2fc-b2c8-435a-8194-a93e126b4800"
	encoded := "-Users-test-proj"
	transcript := `{"type":"summary","slug":"s","sessionId":"x"}
{"cwd":"/Users/test/proj","type":"user","message":{"role":"user","content":"do the thing for me please"},"timestamp":"2026-05-01T10:00:00Z"}
{"type":"assistant","message":{"role":"assistant","content":"ok","usage":{"input_tokens":5,"cache_read_input_tokens":1000,"output_tokens":10}},"timestamp":"2026-05-01T10:01:00Z"}
`
	mdir := filepath.Join(dirs.Archive, "projects", encoded)
	if err := os.MkdirAll(mdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mdir, uuid+".jsonl"), []byte(transcript), 0o644); err != nil {
		t.Fatal(err)
	}
	if withLiveCopy {
		ldir := filepath.Join(dirs.Projects, encoded)
		if err := os.MkdirAll(ldir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ldir, uuid+".jsonl"), []byte(transcript), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dirs, uuid
}

func TestListArchive(t *testing.T) {
	dirs, uuid := buildMirror(t, false)

	groups, bytes := ListArchive(dirs)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	g := groups[0]
	if g.Project != "/Users/test/proj" {
		t.Errorf("project = %q, want /Users/test/proj (from cwd)", g.Project)
	}
	if g.DirName != "proj" {
		t.Errorf("dirName = %q, want proj", g.DirName)
	}
	if len(g.Sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(g.Sessions))
	}
	s := g.Sessions[0]
	if s.ID != uuid {
		t.Errorf("id = %q, want %q", s.ID, uuid)
	}
	if s.LiveCopy {
		t.Error("LiveCopy = true, want false (no live copy created)")
	}
	if s.ContextTokens != 1015 {
		t.Errorf("contextTokens = %d, want 1015", s.ContextTokens)
	}
	if bytes <= 0 {
		t.Errorf("archive bytes = %d, want > 0", bytes)
	}
}

func TestListArchive_LiveCopyFlag(t *testing.T) {
	dirs, _ := buildMirror(t, true)
	groups, _ := ListArchive(dirs)
	if len(groups) != 1 || len(groups[0].Sessions) != 1 {
		t.Fatalf("unexpected shape: %+v", groups)
	}
	if !groups[0].Sessions[0].LiveCopy {
		t.Error("LiveCopy = false, want true (live copy exists)")
	}
}

func TestListArchive_MissingMirror(t *testing.T) {
	dirs := Dirs{
		Projects: t.TempDir(),
		Archive:  filepath.Join(t.TempDir(), "does-not-exist"),
	}
	groups, bytes := ListArchive(dirs)
	if groups != nil {
		t.Errorf("groups = %v, want nil", groups)
	}
	if bytes != 0 {
		t.Errorf("bytes = %d, want 0", bytes)
	}
}

func TestRestoreArchive(t *testing.T) {
	dirs, uuid := buildMirror(t, false)
	encoded := "-Users-test-proj"
	live := filepath.Join(dirs.Projects, encoded, uuid+".jsonl")

	if _, err := os.Stat(live); err == nil {
		t.Fatal("live copy should not exist yet")
	}
	if err := RestoreArchive(dirs, uuid); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("live copy not created: %v", err)
	}
	// Idempotent: restoring again is a no-op success.
	if err := RestoreArchive(dirs, uuid); err != nil {
		t.Errorf("second restore failed: %v", err)
	}
}

func TestRestoreArchive_RejectsPathTraversal(t *testing.T) {
	dirs, _ := buildMirror(t, false)
	for _, bad := range []string{
		"../../etc/passwd",
		"not-a-uuid",
		"7022e2fc-b2c8-435a-8194-a93e126b4800/../../escape",
		"",
	} {
		if err := RestoreArchive(dirs, bad); err == nil {
			t.Errorf("RestoreArchive(%q) = nil, want error", bad)
		}
	}
}

func TestRestoreArchive_UnknownUUID(t *testing.T) {
	dirs, _ := buildMirror(t, false)
	// Well-formed but not present in the mirror.
	err := RestoreArchive(dirs, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for unknown uuid, got nil")
	}
}
