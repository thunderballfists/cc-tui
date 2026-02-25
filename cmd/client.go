package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"cc-tui/tui"
)

func RunClient() {
	home, _ := os.UserHomeDir()
	sockPath := filepath.Join(home, ".claude", "cc-tui.sock")

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		// Auto-start daemon
		daemonBin, _ := os.Executable()
		daemonCmd := exec.Command(daemonBin, "serve")
		daemonCmd.Start()
		// Wait for socket
		for i := 0; i < 40; i++ {
			time.Sleep(250 * time.Millisecond)
			conn, err = net.Dial("unix", sockPath)
			if err == nil {
				break
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot connect to daemon: %v\n", err)
			os.Exit(1)
		}
	}
	defer conn.Close()

	app := tui.NewApp(conn)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
