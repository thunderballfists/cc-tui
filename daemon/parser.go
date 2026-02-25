package daemon

import (
	"bufio"
	"encoding/json"
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

type HistoryEntry struct {
	ID      string
	Project string
	LastTS  int64
}

func LoadHistory(historyPath string) ([]HistoryEntry, error) {
	f, err := os.Open(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	type sessionAcc struct {
		id      string
		project string
		lastTS  int64
	}
	byID := make(map[string]*sessionAcc)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var entry struct {
			SessionID string `json:"sessionId"`
			Project   string `json:"project"`
			Timestamp int64  `json:"timestamp"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.SessionID == "" {
			continue
		}
		acc, ok := byID[entry.SessionID]
		if !ok {
			acc = &sessionAcc{id: entry.SessionID, project: entry.Project}
			byID[entry.SessionID] = acc
		}
		if entry.Timestamp > acc.lastTS {
			acc.lastTS = entry.Timestamp
		}
		if entry.Project != "" {
			acc.project = entry.Project
		}
	}

	result := make([]HistoryEntry, 0, len(byID))
	for _, acc := range byID {
		result = append(result, HistoryEntry{
			ID:      acc.id,
			Project: acc.project,
			LastTS:  acc.lastTS,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTS > result[j].LastTS
	})
	return result, nil
}
