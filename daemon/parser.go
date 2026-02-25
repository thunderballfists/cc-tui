package daemon

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var nonAlphanumHyphen = regexp.MustCompile(`[^a-zA-Z0-9-]`)

func EncodeProjectPath(path string) string {
	return nonAlphanumHyphen.ReplaceAllString(path, "-")
}

func FindSessionUUID(projectPath, projectsDir string) (uuid string, jsonlPath string) {
	encoded := EncodeProjectPath(projectPath)
	projDir := filepath.Join(projectsDir, encoded)

	entries, err := os.ReadDir(projDir)
	if err != nil {
		return "", ""
	}

	type fileInfo struct {
		name  string
		mtime int64
	}
	var jsonls []fileInfo
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".jsonl" {
			info, err := e.Info()
			if err != nil {
				continue
			}
			jsonls = append(jsonls, fileInfo{e.Name(), info.ModTime().UnixMilli()})
		}
	}
	if len(jsonls) == 0 {
		return "", ""
	}

	sort.Slice(jsonls, func(i, j int) bool {
		return jsonls[i].mtime > jsonls[j].mtime
	})

	name := jsonls[0].name
	uuid = name[:len(name)-len(".jsonl")]
	return uuid, filepath.Join(projDir, name)
}
