# Session Archive

Preserve Claude Code session transcripts before its ~30-day deletion sweep, and let cc-tui browse and restore archived sessions.

## Background

Claude Code deletes session transcript `.jsonl` files after ~30 days (confirmed: oldest transcript on disk is exactly 30 days old, zero older). Once deleted, a session can't be resumed or previewed. Users want to recover specific old sessions.

## Approach

Two independent layers:

1. **Mirror** — a launchd/systemd timer runs `rsync -a --ignore-existing` from `~/.claude/projects/` to `~/.claude/cc-tui-archive/projects/` daily. No `--delete`, so transcripts removed from the source survive in the mirror forever. Robust, no custom preservation code.

2. **Archive view** — a cc-tui overlay (press `A`) reads the mirror, lists archived sessions, and restores one by copying its `.jsonl` back into `~/.claude/projects/`. Shows total archive disk usage and opens the archive dir for manual cleanup.

## Decisions

- Retention is "keep everything" — no automatic pruning
- Curation is manual via the OS file manager (archive view opens the directory)
- No starring in v1 (was considered; dropped since nothing is auto-pruned)
- Archiving is a separate launchd job, NOT the daemon — keeps the long-running socket server free of periodic-job failure modes
- Mirror layout matches the source exactly, so restore is a reverse copy to the same relative path

## Dropped for v1

- Starring / favoriting
- Automatic prune of unstarred archives
- Daemon-side fsnotify "archive on first sight"

**Tradeoff:** unbounded disk growth (~1 GB today, slow). Acceptable — local disk; the disk-usage display + open-directory action make manual cleanup easy.

## Mirror Layer

`cc-tui-archive-sync.sh`:
```sh
rsync -a --ignore-existing \
  "$HOME/.claude/projects/" \
  "$HOME/.claude/cc-tui-archive/projects/"
```
- `-a` preserves structure/timestamps; `--ignore-existing` makes incremental runs fast (only new transcripts copy); no `--delete`.
- Layout: `cc-tui-archive/projects/{encoded-project}/{uuid}.jsonl` — identical to source.

`com.cc-tui.archive.plist` (macOS): `StartCalendarInterval` daily + `RunAtLoad`; one-shot job. Linux/WSL: systemd timer unit. Installed by `install.sh` with the same `__INSTALL_DIR__`/`__HOME__` placeholders as the daemon.

## Daemon Protocol

Three new commands on the existing Unix socket:

- **`archive-list`** — walks the mirror, parses each transcript's metadata via existing `LoadSessionMeta`, flags whether a live copy still exists, returns sessions grouped by project + total archive bytes.
- **`archive-restore`** (SessionID = uuid) — copies `cc-tui-archive/projects/{encoded}/{uuid}.jsonl` back to `~/.claude/projects/{encoded}/{uuid}.jsonl`. Idempotent. Validates uuid is a plain UUID (no path traversal).
- **`archive-open`** — `open`/`xdg-open` on `~/.claude/cc-tui-archive`.

Protocol additions:
```go
type Response struct {
    ...
    Archives     []ArchivedGroup `json:"archives,omitempty"`
    ArchiveBytes int64           `json:"archive_bytes,omitempty"`
}
```
`ArchivedGroup` mirrors `ProjectGroup`; each session flagged `LiveCopy bool`.

## TUI Archive View

Overlay opened with `A`, same interception pattern as preview/search/filter.

```
 Archive: 1.2 GB across 487 sessions

 ▾ dih-636-alarm-record
   3575df49 • 114K  │ implement the alarm record…   30d
   4fe1bc34 • 96K   │ fix the retry backoff logic   31d  ✓live
 ...
 ↑↓ navigate  ⏎ restore  p preview  o open dir  esc back
```

- Header: total disk usage + session count
- Rows grouped by project; show uuid/title, context size (`formatTokens`), last-msg snippet, age, `✓live` tag if a live copy exists
- Keys: `↑↓` navigate, `Enter` restore (then flip to `✓live` + refresh tree), `p` preview (conversation loader pointed at mirror), `o` open dir, `Esc` back
- `ArchiveState` struct mirrors `SearchState`; fetched lazily on first `A` via `archiveListMsg`
- Main footer gains `A archive`

## Edge Cases

- Mirror missing → empty list, zero bytes, "No archived sessions yet" message
- Restore target already live → no-op success
- Archived == live (recently mirrored) → shown `✓live`, restore is no-op
- rsync mid-run → `--ignore-existing` adds whole files; `LoadSessionMeta` tolerates parse errors
- Disk full → launchd job fails to its own log; daemon/TUI unaffected
- Path traversal → restore validates uuid format before joining paths

## Testing

- `archive-list` parsing + byte-summing against a temp mirror fixture
- `archive-restore` copies + idempotent + rejects non-UUID SessionID
- `ArchiveState` navigation/scroll bounds

## Files

New: `cc-tui-archive-sync.sh`, `com.cc-tui.archive.plist`, systemd timer, `daemon/archive.go`, `tui/archive.go`
Modified: `install.sh`, `protocol/messages.go`, `daemon/server.go`, `tui/app.go`, `tui/keymap.go`, `tui/styles.go`
