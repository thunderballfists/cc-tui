#!/bin/sh
# cc-tui-archive-sync — mirror Claude Code session transcripts into an archive
# that survives Claude Code's ~30-day deletion sweep.
#
# Uses rsync WITHOUT --delete and WITH --ignore-existing, so:
#   - new transcripts are copied in
#   - transcripts later deleted from the source are KEPT in the mirror
#   - already-mirrored files are skipped (fast incremental runs)
#
# Run on a schedule (launchd on macOS, systemd timer on Linux/WSL). Retention is
# "keep everything" — prune manually via the archive view's "open dir" action.

set -eu

SRC="$HOME/.claude/projects/"
DST="$HOME/.claude/cc-tui-archive/projects/"

mkdir -p "$DST"

# --ignore-existing: never overwrite/re-copy files already in the mirror.
# No --delete: removals in SRC do not propagate to DST.
exec rsync -a --ignore-existing "$SRC" "$DST"
