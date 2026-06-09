package daemon

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"cc-tui/model"
)

// OpenPath opens a path in the OS file manager (Finder / file explorer).
func OpenPath(path string) {
	if path == "" {
		return
	}
	// Ensure the directory exists so the open command doesn't fail.
	_ = os.MkdirAll(path, 0o755)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("/usr/bin/open", path)
	default: // linux / wsl
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}

// uuidRe matches a bare session UUID — used to reject path traversal in restore.
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// archiveProjectsDir is the mirror's projects root: ~/.claude/cc-tui-archive/projects
func archiveProjectsDir(dirs Dirs) string {
	return filepath.Join(dirs.Archive, "projects")
}

// ListArchive walks the archive mirror and returns archived sessions grouped by
// project, plus the total bytes used by the whole archive directory.
func ListArchive(dirs Dirs) ([]model.ArchivedGroup, int64) {
	root := archiveProjectsDir(dirs)
	entries, err := os.ReadDir(root)
	if err != nil {
		// Mirror not created yet (rsync never ran) — empty, zero bytes.
		return nil, 0
	}

	byProject := make(map[string]*model.ArchivedGroup)
	var order []string

	for _, encDir := range entries {
		if !encDir.IsDir() {
			continue
		}
		encPath := filepath.Join(root, encDir.Name())
		files, err := os.ReadDir(encPath)
		if err != nil {
			continue
		}
		for _, fe := range files {
			if fe.IsDir() || filepath.Ext(fe.Name()) != ".jsonl" {
				continue
			}
			uuid := strings.TrimSuffix(fe.Name(), ".jsonl")
			jsonlPath := filepath.Join(encPath, fe.Name())

			meta := LoadSessionMeta(jsonlPath)
			project, dir := projectFromTranscript(jsonlPath)
			if project == "" {
				// Fall back to the encoded dir name if cwd wasn't found.
				project = encDir.Name()
				dir = encDir.Name()
			}

			var size int64
			if info, err := fe.Info(); err == nil {
				size = info.Size()
			}

			// A live (resumable) copy still exists if the source transcript is present.
			liveCopy := false
			if _, err := os.Stat(filepath.Join(dirs.Projects, encDir.Name(), fe.Name())); err == nil {
				liveCopy = true
			}

			sess := model.ArchivedSession{
				ID:            uuid,
				Project:       project,
				Title:         CleanTitle(meta.Title),
				LastMsg:       CleanMessage(meta.LastUserMsg),
				LastActive:    fileModTime(jsonlPath),
				ContextTokens: meta.ContextTokens,
				SizeBytes:     size,
				LiveCopy:      liveCopy,
			}

			g, ok := byProject[project]
			if !ok {
				g = &model.ArchivedGroup{Project: project, DirName: dir}
				byProject[project] = g
				order = append(order, project)
			}
			g.Sessions = append(g.Sessions, sess)
		}
	}

	groups := make([]model.ArchivedGroup, 0, len(byProject))
	for _, p := range order {
		g := byProject[p]
		sort.Slice(g.Sessions, func(i, j int) bool {
			return g.Sessions[i].LastActive.After(g.Sessions[j].LastActive)
		})
		groups = append(groups, *g)
	}
	// Most recently active project first.
	sort.Slice(groups, func(i, j int) bool {
		var li, lj int64
		if len(groups[i].Sessions) > 0 {
			li = groups[i].Sessions[0].LastActive.UnixNano()
		}
		if len(groups[j].Sessions) > 0 {
			lj = groups[j].Sessions[0].LastActive.UnixNano()
		}
		return li > lj
	})

	return groups, dirSize(dirs.Archive)
}

// RestoreArchive copies an archived transcript back into ~/.claude/projects so
// the session becomes resumable again. Idempotent: if the live copy already
// exists, it's a no-op. The uuid is validated to prevent path traversal.
func RestoreArchive(dirs Dirs, uuid string) error {
	if !uuidRe.MatchString(uuid) {
		return os.ErrInvalid
	}

	// Find which encoded project dir holds this uuid in the mirror.
	root := archiveProjectsDir(dirs)
	encDirs, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, ed := range encDirs {
		if !ed.IsDir() {
			continue
		}
		src := filepath.Join(root, ed.Name(), uuid+".jsonl")
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dstDir := filepath.Join(dirs.Projects, ed.Name())
		dst := filepath.Join(dstDir, uuid+".jsonl")
		if _, err := os.Stat(dst); err == nil {
			return nil // already live
		}
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}
		return copyFile(src, dst)
	}
	return os.ErrNotExist
}

// projectFromTranscript reads the cwd field from the first record that has one,
// returning the real project path and its base dir name.
func projectFromTranscript(jsonlPath string) (project, dir string) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	scanned := 0
	for scanner.Scan() && scanned < 50 {
		scanned++
		var d struct {
			Cwd string `json:"cwd"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &d); err != nil {
			continue
		}
		if d.Cwd != "" {
			return d.Cwd, dirName(d.Cwd)
		}
	}
	return "", ""
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func dirSize(path string) int64 {
	var total int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func fileModTime(path string) time.Time {
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}
