#!/bin/sh
set -e

INSTALL_DIR="$(cd "$(dirname "$0")" && pwd)"
HOME_DIR="$HOME"
TOGGLE_SCRIPT="$INSTALL_DIR/cc-tui-toggle.sh"
TMUX_CONF="$HOME_DIR/.tmux.conf"

# Detect platform
OS="$(uname -s)"
IS_WSL=false
if [ "$OS" = "Linux" ] && grep -qi microsoft /proc/version 2>/dev/null; then
  IS_WSL=true
fi

echo "cc-tui installer"
echo "================"
echo ""
echo "Install directory: $INSTALL_DIR"
if [ "$OS" = "Darwin" ]; then
  echo "Platform: macOS (launchd)"
elif $IS_WSL; then
  echo "Platform: WSL/Linux (systemd)"
else
  echo "Platform: Linux (systemd)"
fi
echo ""

# --- Check dependencies ---

if ! command -v go >/dev/null 2>&1; then
  echo "Error: Go is not installed."
  if [ "$OS" = "Darwin" ]; then
    echo "  Install with: brew install go"
  else
    echo "  Install with: sudo apt install golang-go"
    echo "  Or: sudo snap install go --classic"
  fi
  echo "  Or visit: https://go.dev/dl/"
  exit 1
fi

if ! command -v tmux >/dev/null 2>&1; then
  echo "Warning: tmux is not installed."
  if [ "$OS" = "Darwin" ]; then
    echo "  Install with: brew install tmux"
  else
    echo "  Install with: sudo apt install tmux"
  fi
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

# --- Ensure log directory exists ---

mkdir -p "$HOME_DIR/.claude"

# --- Install daemon ---

echo ""

if [ "$OS" = "Darwin" ]; then
  # macOS: launchd
  PLIST_NAME="com.cc-tui.daemon"
  PLIST_SRC="$INSTALL_DIR/$PLIST_NAME.plist"
  PLIST_DST="$HOME_DIR/Library/LaunchAgents/$PLIST_NAME.plist"

  echo "Installing launchd daemon..."

  if [ -f "$PLIST_DST" ]; then
    launchctl unload "$PLIST_DST" 2>/dev/null || true
    echo "  Unloaded existing daemon"
  fi

  mkdir -p "$HOME_DIR/Library/LaunchAgents"
  sed -e "s|__INSTALL_DIR__|$INSTALL_DIR|g" \
      -e "s|__HOME__|$HOME_DIR|g" \
      "$PLIST_SRC" > "$PLIST_DST"

  launchctl load "$PLIST_DST"
  echo "  Daemon installed and started"

else
  # Linux/WSL: systemd user service
  SERVICE_SRC="$INSTALL_DIR/cc-tui-daemon.service"
  SERVICE_DIR="$HOME_DIR/.config/systemd/user"
  SERVICE_DST="$SERVICE_DIR/cc-tui-daemon.service"

  echo "Installing systemd user service..."

  mkdir -p "$SERVICE_DIR"
  sed -e "s|__INSTALL_DIR__|$INSTALL_DIR|g" \
      -e "s|__HOME__|$HOME_DIR|g" \
      "$SERVICE_SRC" > "$SERVICE_DST"

  systemctl --user daemon-reload
  systemctl --user enable cc-tui-daemon.service
  systemctl --user restart cc-tui-daemon.service
  echo "  Service installed and started"

  # WSL: systemd may need lingering enabled
  if $IS_WSL; then
    if ! loginctl show-user "$USER" --property=Linger 2>/dev/null | grep -q "yes"; then
      echo ""
      echo "  Note: To keep the daemon running after you close your terminal,"
      echo "  enable lingering with: sudo loginctl enable-linger $USER"
    fi
  fi
fi

# --- Install archive sync job ---

echo ""
chmod +x "$INSTALL_DIR/cc-tui-archive-sync.sh"

if [ "$OS" = "Darwin" ]; then
  ARCHIVE_PLIST="com.cc-tui.archive"
  ARCHIVE_SRC="$INSTALL_DIR/$ARCHIVE_PLIST.plist"
  ARCHIVE_DST="$HOME_DIR/Library/LaunchAgents/$ARCHIVE_PLIST.plist"

  echo "Installing archive sync (daily launchd job)..."
  if [ -f "$ARCHIVE_DST" ]; then
    launchctl unload "$ARCHIVE_DST" 2>/dev/null || true
  fi
  sed -e "s|__INSTALL_DIR__|$INSTALL_DIR|g" \
      -e "s|__HOME__|$HOME_DIR|g" \
      "$ARCHIVE_SRC" > "$ARCHIVE_DST"
  launchctl load "$ARCHIVE_DST"
  echo "  Archive sync installed (runs daily at 12:00 and at login)"
else
  ARCHIVE_SERVICE_SRC="$INSTALL_DIR/cc-tui-archive.service"
  ARCHIVE_TIMER_SRC="$INSTALL_DIR/cc-tui-archive.timer"
  SERVICE_DIR="$HOME_DIR/.config/systemd/user"

  echo "Installing archive sync (daily systemd timer)..."
  mkdir -p "$SERVICE_DIR"
  sed -e "s|__INSTALL_DIR__|$INSTALL_DIR|g" \
      -e "s|__HOME__|$HOME_DIR|g" \
      "$ARCHIVE_SERVICE_SRC" > "$SERVICE_DIR/cc-tui-archive.service"
  cp "$ARCHIVE_TIMER_SRC" "$SERVICE_DIR/cc-tui-archive.timer"
  systemctl --user daemon-reload
  systemctl --user enable --now cc-tui-archive.timer
  echo "  Archive sync timer installed (runs daily at 12:00)"
fi

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
echo "See README.md for keybindings and configuration tips."
