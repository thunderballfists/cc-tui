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
	groups := cache.Groups()
	t.Logf("loaded %d project groups", len(groups))
	for _, g := range groups {
		t.Logf("  %s: %d sessions (active=%v)", g.DirName, len(g.Sessions), g.Active)
	}
	if len(groups) == 0 {
		t.Error("expected at least 1 project group")
	}
}
