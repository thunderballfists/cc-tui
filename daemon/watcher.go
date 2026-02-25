package daemon

import (
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	watcher  *fsnotify.Watcher
	cache    *Cache
	debounce *time.Timer
}

func NewWatcher(cache *Cache, dirs Dirs) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch key directories
	for _, dir := range []string{dirs.Tasks, dirs.Todos, dirs.Plans} {
		_ = w.Add(dir)
	}
	// Watch history file's parent directory (for history.jsonl changes)
	_ = w.Add(filepath.Dir(dirs.History))

	return &Watcher{watcher: w, cache: cache}, nil
}

func (w *Watcher) Run() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				w.debouncedReload()
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)
		}
	}
}

func (w *Watcher) debouncedReload() {
	if w.debounce != nil {
		w.debounce.Stop()
	}
	w.debounce = time.AfterFunc(500*time.Millisecond, func() {
		if err := w.cache.Reload(); err != nil {
			log.Printf("reload error: %v", err)
		}
	})
}

func (w *Watcher) Close() {
	w.watcher.Close()
}
