package cmd

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"cc-tui/daemon"
)

func RunServe() {
	home, _ := os.UserHomeDir()
	dirs := daemon.DefaultDirs()
	sockPath := filepath.Join(home, ".claude", "cc-tui.sock")
	logPath := filepath.Join(home, ".claude", "cc-tui.log")

	// Set up logging
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	log.Println("cc-tui daemon starting")

	cache := daemon.NewCache(dirs)
	if err := cache.Reload(); err != nil {
		log.Fatalf("initial load failed: %v", err)
	}
	log.Printf("loaded %d sessions", len(cache.Sessions()))

	watcher, err := daemon.NewWatcher(cache, dirs)
	if err != nil {
		log.Fatalf("watcher failed: %v", err)
	}
	defer watcher.Close()
	go watcher.Run()

	server := daemon.NewServer(cache, sockPath)
	if err := server.Start(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
	defer server.Close()

	log.Printf("listening on %s", sockPath)

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down")
}
