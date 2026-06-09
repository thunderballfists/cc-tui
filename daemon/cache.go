package daemon

import (
	"os"
	"path/filepath"
	"sort"
	"sync"

	"cc-tui/model"
)

type Cache struct {
	mu     sync.RWMutex
	groups []model.ProjectGroup
	dirs   Dirs
}

func NewCache(dirs Dirs) *Cache {
	return &Cache{dirs: dirs}
}

func (c *Cache) Reload() error {
	entries, err := LoadHistory(c.dirs.History)
	if err != nil {
		return err
	}

	// Group entries by project
	byProject := make(map[string][]HistoryEntry)
	var projectOrder []string
	for _, e := range entries {
		if e.Project == "" {
			continue
		}
		if _, seen := byProject[e.Project]; !seen {
			projectOrder = append(projectOrder, e.Project)
		}
		byProject[e.Project] = append(byProject[e.Project], e)
	}

	groups := make([]model.ProjectGroup, 0, len(byProject))
	for _, proj := range projectOrder {
		ents := byProject[proj]
		// Sort snapshots by time desc (most recent first)
		sort.Slice(ents, func(i, j int) bool {
			return ents[i].LastTS > ents[j].LastTS
		})

		// Load full data only for the most recent snapshot
		latest := LoadFullSession(ents[0], c.dirs)

		// Build lightweight snapshots for the rest — use each session's own JSONL.
		// Skip sessions whose JSONL is missing: Claude Code deletes transcripts
		// after ~30 days, and a session with no transcript can't be resumed.
		encoded := EncodeProjectPath(proj)
		sessions := make([]model.Session, 0, len(ents))
		sessions = append(sessions, latest)
		for _, e := range ents[1:] {
			jsonlPath := filepath.Join(c.dirs.Projects, encoded, e.ID+".jsonl")
			if _, err := os.Stat(jsonlPath); err != nil {
				continue // transcript gone — not resumable, skip
			}
			meta := LoadSessionMeta(jsonlPath)
			s := model.Session{
				ID:            e.ID,
				Project:       e.Project,
				DirName:       dirName(e.Project),
				LastActive:    timeFromMillis(e.LastTS),
				Slug:          meta.Slug,
				Title:         CleanTitle(meta.Title),
				GitBranch:     meta.GitBranch,
				LastMsg:       CleanMessage(meta.LastUserMsg),
				ContextTokens: meta.ContextTokens,
			}
			sessions = append(sessions, s)
		}

		g := model.ProjectGroup{
			Project:    proj,
			DirName:    latest.DirName,
			Active:     latest.Active,
			PaneID:     latest.PaneID,
			PaneLabel:  latest.PaneLabel,
			LastActive: latest.LastActive,
			Sessions:   sessions,
		}
		groups = append(groups, g)
	}

	// Sort: active first, then by most recent
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Active != groups[j].Active {
			return groups[i].Active
		}
		return groups[i].LastActive.After(groups[j].LastActive)
	})

	if len(groups) > 25 {
		groups = groups[:25]
	}

	c.mu.Lock()
	c.groups = groups
	c.mu.Unlock()
	return nil
}

func (c *Cache) Groups() []model.ProjectGroup {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]model.ProjectGroup, len(c.groups))
	copy(result, c.groups)
	return result
}

func (c *Cache) UpdateActiveStatus(activePanes map[string]PaneInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.groups {
		info, ok := activePanes[c.groups[i].Project]
		c.groups[i].Active = ok
		if len(c.groups[i].Sessions) > 0 {
			c.groups[i].Sessions[0].Active = ok
			if ok {
				c.groups[i].Sessions[0].PaneID = info.PaneID
				c.groups[i].Sessions[0].PaneLabel = info.PaneLabel
				c.groups[i].PaneID = info.PaneID
				c.groups[i].PaneLabel = info.PaneLabel
			} else {
				c.groups[i].Sessions[0].PaneID = ""
				c.groups[i].Sessions[0].PaneLabel = ""
				c.groups[i].PaneID = ""
				c.groups[i].PaneLabel = ""
			}
		}
	}
}
