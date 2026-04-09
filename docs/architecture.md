# Building a TUI Session Manager for Claude Code

Everything an agent (AI or human) needs to replicate or extend cc-tui.

## What cc-tui Is

A **Go-based terminal UI** that runs as a persistent sidebar inside tmux, giving you a live view of all Claude Code sessions across projects. Three components:

1. **Daemon** — background process that watches `~/.claude/projects/` for session data, serves it over a Unix socket
2. **TUI client** — Bubble Tea app that connects to the daemon, renders a tree of sessions, and dispatches tmux actions (open, kill, new)
3. **Toggle script** — shell script bound to a tmux key that opens/closes the sidebar pane

## Technology Stack

| Layer | Tool | Why |
|-------|------|-----|
| Language | **Go** | Single binary, no runtime deps, fast startup |
| TUI framework | **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** (charmbracelet) | Elm-architecture TUI — `Model`, `Update`, `View`. Handles keyboard, mouse, resize |
| Styling | **[Lip Gloss](https://github.com/charmbracelet/lipgloss)** | CSS-like terminal styling (colors, borders, padding) |
| Input widgets | **[Bubbles](https://github.com/charmbracelet/bubbles)** | Pre-built components (text input, help, key bindings) |
| Terminal multiplexer | **tmux** | Pane management, session isolation, keybindings |
| IPC | **Unix domain socket** | Daemon-to-TUI communication via JSON-over-socket |
| Process manager | **launchd** (macOS) / **systemd** (Linux) | Keeps daemon alive, starts on login |

## Project Structure

```
cc-tui/
├── main.go              # Entry point: "serve" → daemon, default → TUI client
├── cmd/
│   ├── client.go        # TUI startup, daemon auto-start, socket probe
│   └── serve.go         # Daemon startup
├── daemon/
│   ├── server.go        # Unix socket server, request router
│   ├── watcher.go       # Filesystem watcher for ~/.claude/projects/
│   ├── parser.go        # Parses Claude Code session JSON files
│   ├── cache.go         # In-memory session cache with TTL
│   ├── conversation.go  # Reads conversation history from session files
│   └── tmux.go          # All tmux interactions (split, resize, kill, find panes)
├── tui/
│   ├── app.go           # Main Bubble Tea model — Update/View loop
│   ├── tree.go          # Tree widget (projects → sessions, expand/collapse)
│   ├── keymap.go        # Key bindings (j/k, enter, n, x, w, /, ?)
│   ├── banner.go        # Gradient header bar with live stats
│   ├── preview.go       # Conversation preview overlay
│   ├── styles.go        # Lip Gloss style definitions
│   ├── help.go          # Help overlay
│   └── timeutil.go      # Relative time formatting
├── model/
│   └── types.go         # Shared data types (ProjectGroup, Session, etc.)
├── protocol/
│   └── protocol.go      # Request/Response JSON schema for socket IPC
├── cc-tui-toggle.sh     # tmux toggle script
├── install.sh           # Cross-platform installer
├── com.cc-tui.daemon.plist  # macOS launchd config
└── cc-tui-daemon.service    # Linux systemd config
```

## Architecture Patterns

### 1. Daemon + Client over Unix Socket

```
┌─────────────┐   JSON/Unix Socket   ┌──────────────┐
│  TUI Client  │ ←──────────────────→ │    Daemon     │
│ (Bubble Tea) │                      │ (background)  │
└─────────────┘                       └───────┬───────┘
                                              │
                                    ┌─────────┴─────────┐
                                    │ ~/.claude/projects/ │
                                    │   (file watcher)    │
                                    └─────────────────────┘
```

- Daemon runs under launchd/systemd, always alive
- Client connects on demand, requests data, disconnects
- Protocol: JSON lines over `~/.claude/cc-tui.sock`
- Client auto-starts daemon if socket unreachable (retry loop with 250ms intervals)

### 2. Bubble Tea Elm Architecture

```go
type App struct { /* state */ }
func (a *App) Init() tea.Cmd       // initial commands (fetch data, start tick)
func (a *App) Update(msg) tea.Cmd  // handle events, return new commands
func (a *App) View() string        // render state to string
```

Key patterns:
- **Async data fetching** via `tea.Cmd` functions that return `tea.Msg`
- **Periodic refresh** via `tea.Tick` (2s interval) that re-fetches the session tree
- **Overlay model** — help, preview, and filter are boolean overlays that intercept key events before the main tree

### 3. tmux Pane Management

This is where most of the bugs lived. Key lessons below.

**Finding panes:** Use `tmux list-panes -F` with format strings to get structured data:
```sh
tmux list-panes -t "$WINDOW" -F '#{pane_id}|#{pane_title}|#{pane_left}|#{pane_width}'
```

**Targeting splits:** Always specify `-t $PANE_ID` explicitly. Without it, tmux defaults to the "active" pane, which is unpredictable when called from a daemon or `run-shell`.

**Rebalancing after split:** `split-window` takes space from the target pane only. Other panes keep their width. You must manually resize if you want even distribution:
```go
avail := windowWidth - sidebarWidth - borders
each := avail / len(workPanes)
for _, pid := range workPanes {
    exec.Command("tmux", "resize-pane", "-t", pid, "-x", fmt.Sprintf("%d", each)).Run()
}
```

## Lessons Learned (Bug Catalog)

### 1. Pane Title Matching Is Fragile — Use File-Based State

**Problem:** Identifying cc-tui by `pane_title` failed because:
- Bubble Tea sets terminal title escape sequences that override tmux's pane title
- Shell prompts can override the title
- Spinner animations add prefixes (`⣾ cc-tui`)

**Solution:** Write the pane ID to a file (`~/.claude/cc-tui-pane-id`) when creating the pane, read it back when toggling. Verify the pane still exists before acting.

```sh
# Create
echo "$PANE_ID" > "$PIDFILE"
# Toggle off — verify first
if tmux display-message -t "$SAVED_ID" -p '#{pane_id}' >/dev/null 2>&1; then
  tmux kill-pane -t "$SAVED_ID"
fi
```

### 2. Daemon Runs Under Minimal PATH

**Problem:** `tmux`, `go`, `node` not found when daemon runs commands, because launchd/systemd don't load shell profiles.

**Solution:** Use absolute paths for all external binaries in daemon code:
```go
const (
    tmuxBin  = "/usr/local/bin/tmux"
    pgrepBin = "/usr/bin/pgrep"
    psBin    = "/bin/ps"
)
```

### 3. `run-shell` Context Is Not the Active Pane

**Problem:** `tmux display-message -p '#{pane_id}'` inside a `run-shell` script returns the pane that triggered the keybinding, not any newly created pane. This caused rebalance logic to exclude the wrong pane.

**Solution:** Identify panes by position (`#{pane_left}`) or by exclusion (known pane IDs), not by "active" state.

### 4. split-window Target Determines Which Pane Loses Space

**Problem:** `split-window -hb` without `-t` splits the active pane. When the daemon runs this, "active" could be the cc-tui sidebar itself, causing it to get split in half.

**Solution:** Always find the correct target pane explicitly:
```go
// Find rightmost non-cc-tui pane in the window
func findTargetPane() string {
    // list panes, filter out cc-tui, pick highest pane_left
}
```

### 5. Pipe Subshells Can Silently Fail

**Problem:** `echo "$PANES" | while read pid; do tmux kill-pane -t "$pid"; done` — the `while` runs in a subshell in `/bin/sh`. Variable assignments inside don't propagate.

**Solution:** Use `for` loops with command substitution:
```sh
for pid in $(tmux list-panes -a -F '...'); do
  tmux kill-pane -t "$pid"
done
```

### 6. Window Scope Matters

**Problem:** `-a` flag lists panes across ALL windows/sessions. Daemon code finding "the active pane" across all sessions picked panes in the wrong window.

**Solution:** First find the relevant window, then scope all queries to it:
```go
func findCCTuiWindow() string { /* find session:window containing cc-tui */ }
// Then: list-panes -t "$window" (not -a)
```

## How to Build a Similar App

### Phase 1: Data Layer
1. Identify where the target app stores its data (Claude Code uses `~/.claude/projects/*/`)
2. Write a parser for the session/state files
3. Build a file watcher (Go: `fsnotify`, Python: `watchdog`)
4. Cache parsed state in memory with TTL

### Phase 2: Daemon
1. Unix socket server with JSON protocol
2. Request types: `tree` (list all), `conversation` (get details), `action` (open/kill/new)
3. Register with launchd/systemd for persistence
4. Use absolute paths for all external commands

### Phase 3: TUI
1. Bubble Tea app with tree navigation
2. Key bindings: navigate, expand/collapse, open, new, kill, filter, preview
3. Periodic refresh via `tea.Tick`
4. Overlay pattern for help/preview/filter modes

### Phase 4: tmux Integration
1. Toggle script that opens/closes the sidebar
2. Track pane ID in a file, not by title
3. Target panes explicitly by ID, never rely on "active"
4. Rebalance work panes after splits
5. Install script that wires up keybindings

## Key Dependencies (Go)

```
github.com/charmbracelet/bubbletea    # TUI framework
github.com/charmbracelet/bubbles      # Input components
github.com/charmbracelet/lipgloss     # Terminal styling
github.com/fsnotify/fsnotify          # File watching (if used)
```

## Testing

- `daemon/parser_test.go` — unit tests for session file parsing
- `daemon/cache_test.go` — cache expiry and update tests
- Manual testing: tmux integration requires a live tmux session — no practical way to unit test pane management
