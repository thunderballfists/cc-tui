#!/bin/sh
# Find ALL panes with cc-tui title (exact match or with spinner prefix)
PANE_IDS=$(tmux list-panes -a -F '#{pane_id}|#{pane_title}' \
  | awk -F'|' '$2 == "cc-tui" || $2 ~ / cc-tui$/ {print $1}')
if [ -n "$PANE_IDS" ]; then
  echo "$PANE_IDS" | while read -r pid; do
    tmux kill-pane -t "$pid" 2>/dev/null
  done
else
  tmux split-window -hb -l 45 \
    "tmux select-pane -T cc-tui; ~/.config/tmux/cc-tui/cc-tui"
fi
