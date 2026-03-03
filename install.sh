#!/bin/sh
set -e

INSTALL_DIR="$(cd "$(dirname "$0")" && pwd)"
HOME_DIR="$HOME"
PLIST_NAME="com.cc-tui.daemon"
PLIST_SRC="$INSTALL_DIR/$PLIST_NAME.plist"
PLIST_DST="$HOME_DIR/Library/LaunchAgents/$PLIST_NAME.plist"
TOGGLE_SCRIPT="$INSTALL_DIR/cc-tui-toggle.sh"
TMUX_CONF="$HOME_DIR/.tmux.conf"

echo "cc-tui installer"
echo "================"
echo ""
echo "Install directory: $INSTALL_DIR"
echo ""

# --- Check dependencies ---

if ! command -v go >/dev/null 2>&1; then
  echo "Error: Go is not installed."
  echo "  Install with: brew install go"
  echo "  Or visit: https://go.dev/dl/"
  exit 1
fi

if ! command -v tmux >/dev/null 2>&1; then
  echo "Warning: tmux is not installed."
  echo "  Install with: brew install tmux"
  echo "  cc-tui requires tmux to function."
  printf "Continue anyway? [y/N] "
  read -r reply
  case "$reply" in
    [yY]*) ;;
    *) exit 1 ;;
  esac
fi

# --- Build ---

echo "Building cc-tui..."
cd "$INSTALL_DIR"
go build -o cc-tui .
echo "  Built: $INSTALL_DIR/cc-tui"

# --- Install daemon ---

echo ""
echo "Installing launchd daemon..."

# Unload existing daemon if present
if [ -f "$PLIST_DST" ]; then
  launchctl unload "$PLIST_DST" 2>/dev/null || true
  echo "  Unloaded existing daemon"
fi

# Create plist from template
mkdir -p "$HOME_DIR/Library/LaunchAgents"
sed -e "s|__INSTALL_DIR__|$INSTALL_DIR|g" \
    -e "s|__HOME__|$HOME_DIR|g" \
    "$PLIST_SRC" > "$PLIST_DST"

# Ensure log directory exists
mkdir -p "$HOME_DIR/.claude"

launchctl load "$PLIST_DST"
echo "  Daemon installed and started"

# --- Toggle script ---

echo ""
chmod +x "$TOGGLE_SCRIPT"
echo "Toggle script: $TOGGLE_SCRIPT"

# --- tmux keybinding ---

echo ""
if [ -f "$TMUX_CONF" ] && grep -q "cc-tui-toggle" "$TMUX_CONF"; then
  echo "tmux keybinding already configured"
else
  echo "Adding tmux keybinding to $TMUX_CONF..."
  printf '\n# cc-tui session manager toggle (prefix + s)\nbind s run-shell "%s"\n' "$TOGGLE_SCRIPT" >> "$TMUX_CONF"
  echo "  Added: bind s run-shell \"$TOGGLE_SCRIPT\""
  echo "  Reload tmux config with: tmux source ~/.tmux.conf"
fi

# --- Done ---

echo ""
echo "Done! cc-tui is installed and running."
echo ""
echo "Usage:"
echo "  In tmux, press <prefix> s to toggle the session panel"
echo "  Or run: $INSTALL_DIR/cc-tui"
echo ""
echo "See README.md for iTerm2 keybindings and configuration tips."
