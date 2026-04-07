#!/bin/sh
export PATH="/usr/local/bin:$PATH"
PIDFILE="$HOME/.claude/cc-tui-pane-id"
CUR_WIN=$(tmux display-message -p '#{window_id}')
ACTIVE_PANE=$(tmux display-message -p '#{pane_id}')

# Read tracked pane ID and verify it still exists
CC_PANE=""
if [ -f "$PIDFILE" ]; then
  SAVED_ID=$(cat "$PIDFILE")
  if tmux display-message -t "$SAVED_ID" -p '#{pane_id}' >/dev/null 2>&1; then
    CC_PANE="$SAVED_ID"
  else
    rm -f "$PIDFILE"
  fi
fi

if [ -n "$CC_PANE" ]; then
  # Toggle off — kill cc-tui and rebalance remaining panes
  tmux kill-pane -t "$CC_PANE" 2>/dev/null
  rm -f "$PIDFILE"
  tmux select-layout -t "$CUR_WIN" -E
else
  # Open cc-tui at the left edge of the window, fixed 45 cols
  LEFT_PANE=$(tmux list-panes -t "$CUR_WIN" -F '#{pane_left}|#{pane_id}' \
    | sort -t'|' -k1 -n | head -1 | cut -d'|' -f2)
  tmux split-window -hb -t "$LEFT_PANE" -l 45 \
    "tmux select-pane -T cc-tui; ~/.config/tmux/cc-tui/cc-tui"
  # Record the new pane ID (leftmost pane after split)
  CC_PANE=$(tmux list-panes -t "$CUR_WIN" -F '#{pane_left}|#{pane_id}' \
    | sort -t'|' -k1 -n | head -1 | cut -d'|' -f2)
  echo "$CC_PANE" > "$PIDFILE"
  # Rebalance non-cc-tui panes to share remaining width evenly
  OTHERS=$(tmux list-panes -t "$CUR_WIN" -F '#{pane_id}' \
    | grep -v "^${CC_PANE}\$")
  NUM_OTHERS=$(echo "$OTHERS" | wc -l | tr -d ' ')
  if [ "$NUM_OTHERS" -gt 1 ]; then
    WIN_WIDTH=$(tmux display-message -t "$CUR_WIN" -p '#{window_width}')
    AVAIL=$((WIN_WIDTH - 45 - NUM_OTHERS))
    EACH=$((AVAIL / NUM_OTHERS))
    for pid in $OTHERS; do
      tmux resize-pane -t "$pid" -x "$EACH"
    done
  fi
  tmux select-pane -t "$ACTIVE_PANE"
fi
