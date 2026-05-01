# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```sh
go build -o cc-tui .          # Build binary
go test ./daemon -v            # Run all tests (only daemon package has tests)
go test ./daemon -run TestLoadPlan -v  # Run a single test
```

After building, restart the daemon to pick up changes:
```sh
# macOS
launchctl stop com.cc-tui.daemon && launchctl start com.cc-tui.daemon

# Linux/WSL
systemctl --user restart cc-tui-daemon
```

Daemon logs: `~/.claude/cc-tui.log` (also `cc-tui-stdout.log`, `cc-tui-stderr.log`)

## Architecture

cc-tui is a daemon+client system for browsing Claude Code sessions inside tmux.

**Daemon** (`cc-tui serve`) — long-running background process managed by launchd (macOS) or systemd (Linux/WSL). It watches `~/.claude/` via fsnotify, maintains an in-memory cache of sessions, polls tmux every 2s for active claude panes, and serves data over a Unix socket at `~/.claude/cc-tui.sock`.

**Client** (`cc-tui` with no args) — ephemeral Bubble Tea TUI. Connects to the daemon socket, renders a navigable tree of projects/sessions, and sends action commands (open, new window, new session) back to the daemon which executes tmux splits.

**Protocol** — JSON request/response over Unix domain socket. Four commands:
- `tree` → snapshot of all project groups
- `subscribe` → streaming updates every 2s (keeps connection open)
- `action` → execute tmux command (open/window/new)
- `conversation` → load a session's JSONL messages for preview

### Package layout

- **`cmd/`** — Entry points: `serve.go` (daemon startup), `client.go` (TUI startup with auto-daemon-launch)
- **`daemon/`** — Server, cache, file watcher, JSONL/plan/task parser, tmux integration. This is where most logic lives.
- **`tui/`** — Bubble Tea app using Elm architecture (Init/Update/View). TreeModel handles navigation over a flattened visible-node array. Overlay modes (help, preview, filter) intercept input before the tree.
- **`model/`** — Shared data types: Session, ProjectGroup, Plan, Task, Todo, ConvMessage
- **`protocol/`** — Request/Response message structs

### Key patterns

- **Debounced file watching**: Watcher coalesces rapid file changes into a single cache reload after 500ms of quiet.
- **Active pane detection**: Daemon runs `tmux list-panes -a` + `pgrep` every 2s to find running claude processes and mark sessions as active.
- **Auto-start**: Client probes the socket on startup; if unreachable, spawns `cc-tui serve` in background and retries (40x, 250ms apart).
- **Tree as flat array**: TreeModel flattens the hierarchical node tree into a `visible` slice for O(1) cursor movement and scroll offset calculation.

### Data sources (all under `~/.claude/`)

| Path | Content |
|------|---------|
| `history.jsonl` | Global session log with timestamps |
| `projects/{encoded-path}/{uuid}.jsonl` | Per-session conversation data |
| `tasks/{uuid}/*.json` | Task definitions |
| `todos/{uuid}-agent-*.json` | Todo items |
| `plans/{slug}.md` | Implementation plans (parsed from markdown headings) |

### Supporting files

- `install.sh` — Cross-platform installer (detects macOS/Linux/WSL, builds binary, installs daemon, patches tmux.conf)
- `cc-tui-toggle.sh` — tmux script to toggle the TUI sidebar (45-col left split with pane rebalancing)
- `com.cc-tui.daemon.plist` — macOS launchd config (placeholders: `__INSTALL_DIR__`, `__HOME__`)
- `cc-tui-daemon.service` — Linux/WSL systemd user service (same placeholders)
