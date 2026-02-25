package daemon

import (
	"cc-tui/model"
	"sort"
	"sync"
)

type Cache struct {
	mu       sync.RWMutex
	sessions []model.Session
	dirs     Dirs
}

func NewCache(dirs Dirs) *Cache {
	return &Cache{dirs: dirs}
}

func (c *Cache) Reload() error {
	entries, err := LoadHistory(c.dirs.History)
	if err != nil {
		return err
	}

	sessions := make([]model.Session, 0, len(entries))
	for _, e := range entries {
		s := LoadFullSession(e, c.dirs)
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Active != sessions[j].Active {
			return sessions[i].Active
		}
		return sessions[i].LastActive.After(sessions[j].LastActive)
	})

	if len(sessions) > 25 {
		sessions = sessions[:25]
	}

	c.mu.Lock()
	c.sessions = sessions
	c.mu.Unlock()
	return nil
}

func (c *Cache) Sessions() []model.Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]model.Session, len(c.sessions))
	copy(result, c.sessions)
	return result
}

func (c *Cache) UpdateActiveStatus(activePanes map[string]PaneInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.sessions {
		info, ok := activePanes[c.sessions[i].Project]
		c.sessions[i].Active = ok
		if ok {
			c.sessions[i].PaneID = info.PaneID
			c.sessions[i].PaneLabel = info.PaneLabel
		} else {
			c.sessions[i].PaneID = ""
			c.sessions[i].PaneLabel = ""
		}
	}
}
