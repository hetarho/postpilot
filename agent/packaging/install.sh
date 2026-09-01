#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
AGENT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
APP_DIR="$HOME/Library/Application Support/Postpilot Agent"
BIN_DIR="$APP_DIR/bin"
LAUNCH_LABEL="com.postpilot.publishing-agent"
LAUNCH_DOMAIN="gui/$(id -u)"
LAUNCH_PLIST="$HOME/Library/LaunchAgents/$LAUNCH_LABEL.plist"

# A previous package may have installed the retired KeepAlive daemon. Stop it
# before compiling or replacing anything so old code cannot keep polling from
# memory during the transition.
if /bin/launchctl print "$LAUNCH_DOMAIN/$LAUNCH_LABEL" >/dev/null 2>&1; then
  /bin/launchctl bootout "$LAUNCH_DOMAIN/$LAUNCH_LABEL"
fi
rm -f "$LAUNCH_PLIST"

mkdir -p "$BIN_DIR"
chmod 700 "$APP_DIR" "$BIN_DIR"
(cd "$AGENT_DIR" && go build -trimpath -o "$BIN_DIR/postpilot-agent" ./cmd/postpilot-agent)
printf '%s\n' "Installed $BIN_DIR/postpilot-agent"
printf '%s\n' "Any previous Postpilot publishing LaunchAgent was stopped and removed."
printf '%s\n' "The deterministic Naver publisher is still under implementation; do not install the LaunchAgent yet."
