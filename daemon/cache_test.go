package daemon

import "testing"

func TestCacheReloadReal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dirs := DefaultDirs()
	cache := NewCache(dirs)
	if err := cache.Reload(); err != nil {
		t.Fatal(err)
	}
	sessions := cache.Sessions()
	t.Logf("loaded %d sessions", len(sessions))
	for _, s := range sessions {
		t.Logf("  %s: %s (active=%v)", s.DirName, s.Title, s.Active)
	}
	if len(sessions) == 0 {
		t.Error("expected at least 1 session")
	}
}
