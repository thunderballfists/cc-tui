# CC-TUI: Combined Session Manager & Dashboard

## Overview

A single Go binary that replaces the existing fzf-based session manager
(cc-sessions-*.sh, cc-session-data.py) and dashboard (cc-dashboard.py,
cc-dashboard-run.sh) with a Bubble Tea TUI backed by an always-running daemon.

## Architecture

Single binary, two modes:

- `cc-tui serve` — daemon. Watches `~/.claude/` with fsnotify, keeps parsed
  session/task/plan/todo data in memory, serves over unix socket.
- `cc-tui` — client TUI. Connects to daemon via unix socket, renders tree
  with Bubble Tea. Auto-starts daemon if not running.

IPC over `~/.claude/cc-tui.sock`, JSON-encoded:

- `{"cmd":"tree"}` — full tree snapshot (on connect)
- `{"cmd":"subscribe"}` — stream of incremental updates
- `{"cmd":"action","type":"open|window|new","session_id":"..."}` — daemon
  executes tmux commands

Daemon manages tmux interaction (pane detection, split/new-window) so the
client is purely a display layer.

## Tree Structure

```
▶ traffic-api             ● ACTIVE  2m ago
  ├─ Plan: REST endpoints           3/8 ███░░
  │  ├─ ☑ Set up project structure
  │  ├─ ☑ Define data models
  │  ├─ ☐ Implement CRUD endpoints      ← wip
  │  ├─ ☐ Add authentication
  │  └─ +4 more...
  ├─ Tasks                           2/5
  │  └─ ☐ Write integration tests       ← next
  └─ Todos                           1/3

▷ mapbox-windy-gl         ○ 3h ago
    Plan: WebGL wind viz  12/24  ████████░░

▷ claude-code-settings    ○ 1d ago
```

Node types:

- **Session** (top-level): project name, active indicator (● green / ○ dim),
  recency. ▶ expanded, ▷ collapsed. Collapsed shows inline summary.
- **Category** (Plan/Tasks/Todos): label + progress count + mini bar.
- **Leaf** (step/task/todo): checkbox (☑ green done, ☐ yellow wip, ☐ dim
  pending) + text.

Active sessions sorted first, then by recency.

## Interaction

Navigation:
- ↑/↓ or j/k — move cursor
- ← — collapse (or jump to parent)
- → — expand
- Home/End or g/G — top/bottom
- / — fuzzy filter

Actions on session nodes:
- Enter — switch to (active → focus pane, inactive → resume in split)
- w — open in new tmux window
- n — new `claude --dangerously-skip-permissions` in project dir
- x — kill active session pane

Mouse:
- Click — expand/collapse
- Double-click or Cmd-click — open session

Global:
- q/Esc — close client (daemon stays)
- r — force refresh
- ? — help overlay

## Data Sources

- `~/.claude/history.jsonl` — session metadata (project path, slug, timestamps)
- `~/.claude/tasks/{session-uuid}/*.json` — task files per session
- `~/.claude/todos/{session-uuid}-agent-*.json` — todo files
- `~/.claude/plans/{slug}.md` — plan markdown files
- tmux pane polling (2s interval) — active session detection via pgrep

Path encoding: `re.sub(r"[^a-zA-Z0-9-]", "-", path)` equivalent in Go for
matching CC's directory naming.

Plan step extraction patterns:
1. `## Step N:` headings
2. Implementation order table rows (`| N | desc |`)
3. Numbered lists under step sections
4. Fallback to actionable `##` headings

Step completion: cross-reference plan steps with completed/in-progress tasks
and todos by 4+ character keyword overlap (min 2 matching words).

## Project Structure

```
~/.config/tmux/cc-tui/
├── go.mod
├── main.go                 # subcommand dispatch
├── cmd/
│   ├── serve.go            # daemon entry
│   └── client.go           # TUI entry
├── daemon/
│   ├── watcher.go          # fsnotify
│   ├── parser.go           # file parsing
│   ├── cache.go            # in-memory store
│   ├── tmux.go             # pane detection, tmux commands
│   └── server.go           # unix socket server
├── tui/
│   ├── app.go              # root Bubble Tea model
│   ├── tree.go             # tree component
│   ├── keymap.go           # key bindings
│   ├── mouse.go            # click handling
│   ├── styles.go           # Lip Gloss
│   └── help.go             # help overlay
├── protocol/
│   └── messages.go         # shared IPC types
└── model/
    ├── session.go
    ├── plan.go
    ├── task.go
    └── todo.go
```

Dependencies:
- charmbracelet/bubbletea
- charmbracelet/lipgloss
- charmbracelet/bubbles (help, key, viewport)
- fsnotify/fsnotify

Custom tree component — our node types, progress bars, checkboxes, mouse
double-click, and inline collapsed summaries are specific enough to warrant
a custom implementation over wrapping a generic community tree.

## Daemon Lifecycle

- launchd plist at `~/Library/LaunchAgents/com.cc-tui.daemon.plist`
- KeepAlive: true, runs on login
- Logs to `~/.claude/cc-tui.log`
- Client auto-starts daemon if socket missing

## tmux Integration

- Cmd+D (User9) toggles client via `cc-tui-toggle.sh`
- Toggle: kill client pane if exists, else spawn split

## Migration

Files removed after cc-tui works:
- cc-sessions-core.sh, cc-sessions.sh, cc-sessions-dock.sh
- cc-session-data.py
- cc-dashboard.py, cc-dashboard-run.sh, cc-dashboard-toggle.sh
- cc-dock-toggle.sh

Files modified:
- tmux.conf.local — consolidate keybindings to User9 → cc-tui-toggle.sh
- pane-menu.sh — update references

Freed: Cmd+S (User0) becomes available.
