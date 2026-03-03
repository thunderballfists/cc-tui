package daemon

import (
	"os/exec"
	"strings"
)

// Use absolute paths — daemon runs under launchd with minimal PATH
const (
	tmuxBin  = "/usr/local/bin/tmux"
	pgrepBin = "/usr/bin/pgrep"
	psBin    = "/bin/ps"
)

type PaneInfo struct {
	PaneID    string
	PaneLabel string
}

func GetActivePanes() map[string]PaneInfo {
	out, err := exec.Command(tmuxBin, "list-panes", "-a", "-F",
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
	out, err := exec.Command(pgrepBin, "-P", pid, "-f", "claude").Output()
	if err == nil && len(out) > 0 {
		return true
	}
	out, err = exec.Command(psBin, "-o", "command=", "-p", pid).Output()
	return err == nil && strings.Contains(string(out), "claude")
}

// findTargetPane finds a non-cc-tui pane to split next to.
// Falls back to current active pane if nothing better found.
func findTargetPane() string {
	out, err := exec.Command(tmuxBin, "list-panes", "-a", "-F",
		"#{pane_id}|#{pane_pid}|#{pane_title}|#{pane_active}|#{window_active}|#{session_attached}").Output()
	if err != nil {
		return ""
	}
	// Find the active pane in the attached session that isn't cc-tui
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}
		paneID, _, title, paneActive, winActive, attached := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]
		if attached == "1" && winActive == "1" && paneActive == "1" && title != "cc-tui" {
			return paneID
		}
	}
	// Fallback: any non-cc-tui pane in attached session
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}
		paneID, _, title, _, _, attached := parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]
		if attached == "1" && title != "cc-tui" {
			return paneID
		}
	}
	return ""
}

// loginShell wraps a command so it runs in a login shell (loads ~/.zshrc, nvm, etc.)
func loginShell(cmd string) string {
	return "/bin/zsh -li -c " + "'" + cmd + "'"
}

// TmuxSplitShell opens a split pane running cmd through a login shell,
// so the user's PATH (nvm, etc.) is available.
func TmuxSplitShell(dir, cmd string) error {
	target := findTargetPane()
	base := []string{"split-window", "-h"}
	if target != "" {
		base = append(base, "-t", target)
	}
	base = append(base, "-c", dir, loginShell(cmd))
	return exec.Command(tmuxBin, base...).Run()
}

// TmuxNewWindowShell opens a new window running cmd through login shell.
func TmuxNewWindowShell(dir, cmd string) error {
	return exec.Command(tmuxBin, "new-window", "-c", dir, loginShell(cmd)).Run()
}

func TmuxSwitchToPane(paneLabel string) error {
	_ = exec.Command(tmuxBin, "switch-client", "-t", paneLabel).Run()
	return exec.Command(tmuxBin, "select-pane", "-t", paneLabel).Run()
}
