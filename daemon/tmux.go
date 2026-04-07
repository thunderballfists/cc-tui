package daemon

import (
	"fmt"
	"os/exec"
	"strconv"
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

func isCCTui(title string) bool {
	return title == "cc-tui" || strings.HasSuffix(title, " cc-tui")
}

// findCCTuiWindow returns the session:window target containing the cc-tui pane.
func findCCTuiWindow() string {
	out, err := exec.Command(tmuxBin, "list-panes", "-a", "-F",
		"#{pane_title}|#{session_name}:#{window_index}").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 && isCCTui(parts[0]) {
			return parts[1]
		}
	}
	return ""
}

// findTargetPane finds a non-cc-tui pane in the same window as cc-tui to split.
// Picks the rightmost work pane so the new pane appears at the far right.
func findTargetPane() string {
	win := findCCTuiWindow()
	if win == "" {
		// No cc-tui window; fall back to active pane in attached session
		out, _ := exec.Command(tmuxBin, "list-panes", "-a", "-F",
			"#{pane_id}|#{pane_title}|#{pane_active}|#{window_active}|#{session_attached}").Output()
		if out == nil {
			return ""
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			parts := strings.SplitN(line, "|", 5)
			if len(parts) == 5 && parts[2] == "1" && parts[3] == "1" && parts[4] == "1" {
				return parts[0]
			}
		}
		return ""
	}

	// List panes in the cc-tui window, pick the rightmost non-cc-tui pane
	out, err := exec.Command(tmuxBin, "list-panes", "-t", win, "-F",
		"#{pane_id}|#{pane_title}|#{pane_left}").Output()
	if err != nil {
		return ""
	}

	bestID := ""
	bestLeft := -1
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		if isCCTui(parts[1]) {
			continue
		}
		left, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}
		if left > bestLeft {
			bestLeft = left
			bestID = parts[0]
		}
	}
	return bestID
}

// loginShell wraps a command so it runs in a login shell (loads ~/.zshrc, nvm, etc.)
func loginShell(cmd string) string {
	return "/bin/zsh -li -c " + "'" + cmd + "'"
}

// TmuxSplitShell opens a split pane running cmd through a login shell,
// so the user's PATH (nvm, etc.) is available.
// After splitting, rebalances non-cc-tui panes so they share width evenly.
func TmuxSplitShell(dir, cmd string) error {
	target := findTargetPane()
	base := []string{"split-window", "-h"}
	if target != "" {
		base = append(base, "-t", target)
	}
	base = append(base, "-c", dir, loginShell(cmd))
	if err := exec.Command(tmuxBin, base...).Run(); err != nil {
		return err
	}
	rebalanceWorkPanes()
	return nil
}

// rebalanceWorkPanes distributes width evenly among non-cc-tui panes
// in the cc-tui window, preserving the cc-tui sidebar width.
func rebalanceWorkPanes() {
	win := findCCTuiWindow()
	if win == "" {
		return
	}

	winW, err := exec.Command(tmuxBin, "display-message", "-t", win, "-p", "#{window_width}").Output()
	if err != nil {
		return
	}
	windowWidth, err := strconv.Atoi(strings.TrimSpace(string(winW)))
	if err != nil {
		return
	}

	out, err := exec.Command(tmuxBin, "list-panes", "-t", win, "-F", "#{pane_id}|#{pane_title}").Output()
	if err != nil {
		return
	}

	var workPanes []string
	ccTuiWidth := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) < 2 {
			continue
		}
		if isCCTui(parts[1]) {
			ccTuiWidth = 45
		} else {
			workPanes = append(workPanes, parts[0])
		}
	}

	if len(workPanes) < 2 {
		return
	}

	totalPanes := len(workPanes)
	if ccTuiWidth > 0 {
		totalPanes++
	}
	borders := totalPanes - 1
	avail := windowWidth - ccTuiWidth - borders
	each := avail / len(workPanes)
	if each < 20 {
		return
	}

	for _, pid := range workPanes {
		_ = exec.Command(tmuxBin, "resize-pane", "-t", pid, "-x", fmt.Sprintf("%d", each)).Run()
	}
}

// TmuxNewWindowShell opens a new window running cmd through login shell.
func TmuxNewWindowShell(dir, cmd string) error {
	return exec.Command(tmuxBin, "new-window", "-c", dir, loginShell(cmd)).Run()
}

func TmuxSwitchToPane(paneLabel string) error {
	_ = exec.Command(tmuxBin, "switch-client", "-t", paneLabel).Run()
	return exec.Command(tmuxBin, "select-pane", "-t", paneLabel).Run()
}
