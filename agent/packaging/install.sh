#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
AGENT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
APP_DIR="$HOME/Library/Application Support/Postpilot Agent"
BIN_DIR="$APP_DIR/bin"
LAUNCH_LABEL="com.postpilot.publishing-agent"
LAUNCH_DOMAIN="gui/$(id -u)"
LAUNCH_PLIST="$HOME/Library/LaunchAgents/$LAUNCH_LABEL.plist"

# Stop the previous signed binary before replacing it so two versions never poll
# the same account concurrently.
if /bin/launchctl print "$LAUNCH_DOMAIN/$LAUNCH_LABEL" >/dev/null 2>&1; then
  /bin/launchctl bootout "$LAUNCH_DOMAIN/$LAUNCH_LABEL"
fi
rm -f "$LAUNCH_PLIST"

mkdir -p "$BIN_DIR"
chmod 700 "$APP_DIR" "$BIN_DIR"
(cd "$AGENT_DIR" && go build -trimpath -o "$BIN_DIR/postpilot-agent" ./cmd/postpilot-agent)
chmod 700 "$BIN_DIR/postpilot-agent"
"$BIN_DIR/postpilot-agent" install
printf '%s\n' "Installed $BIN_DIR/postpilot-agent"
printf '%s\n' "Installed and loaded the per-user Postpilot publishing LaunchAgent."
