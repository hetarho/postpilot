#!/bin/sh
set -eu

APP_DIR="$HOME/Library/Application Support/Postpilot Agent"
BIN="$APP_DIR/bin/postpilot-agent"
if [ -x "$BIN" ]; then "$BIN" uninstall; fi
printf '%s\n' "The LaunchAgent was removed."
printf '%s\n' "For safety, this script kept browser profiles, config, logs, and Keychain credentials. Delete each explicitly only after checking which Postpilot accounts use it."
