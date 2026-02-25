package daemon

import (
	"os/exec"
	"strings"
)

type PaneInfo struct {
	PaneID    string
	PaneLabel string
}

func GetActivePanes() map[string]PaneInfo {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{pane_id}|#{pane_pid}|#{pane_current_path}|#{session_name}:#{window_index}.#{pane_index}").Output()
	if err != nil {
		return nil
	}

	result := make(map[string]PaneInfo)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		paneID, panePID, panePath, paneLabel := parts[0], parts[1], parts[2], parts[3]
		if isClaude(panePID) {
			result[panePath] = PaneInfo{PaneID: paneID, PaneLabel: paneLabel}
		}
	}
	return result
}

func isClaude(pid string) bool {
	out, err := exec.Command("pgrep", "-P", pid, "-f", "claude").Output()
	if err == nil && len(out) > 0 {
		return true
	}
	out, err = exec.Command("ps", "-o", "command=", "-p", pid).Output()
	return err == nil && strings.Contains(string(out), "claude")
}

func TmuxSplitPane(dir, cmd string) error {
	return exec.Command("tmux", "split-window", "-h", "-c", dir, cmd).Run()
}

func TmuxNewWindow(dir, cmd string) error {
	return exec.Command("tmux", "new-window", "-c", dir, cmd).Run()
}

func TmuxSwitchToPane(paneLabel string) error {
	_ = exec.Command("tmux", "switch-client", "-t", paneLabel).Run()
	return exec.Command("tmux", "select-pane", "-t", paneLabel).Run()
}
