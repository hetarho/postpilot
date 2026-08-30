#!/bin/sh
set -eu

# Hermes is installed from its official moving installer so setup verifies the
# implementation-time latest compatible release instead of trusting this repository's
# checked release note. Skip the global interactive wizard: Postpilot creates one named
# profile per connection, and that isolated profile owns its model authentication.
# The capability probe still gates arming each connection.
curl -fsSL https://hermes-agent.nousresearch.com/install.sh | bash -s -- --skip-setup

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
AGENT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
APP_DIR="$HOME/Library/Application Support/Postpilot Agent"
BIN_DIR="$APP_DIR/bin"
SHARE_DIR="$APP_DIR/share/postpilot-agent"
mkdir -p "$BIN_DIR" "$SHARE_DIR"
chmod 700 "$APP_DIR" "$BIN_DIR" "$SHARE_DIR"
(cd "$AGENT_DIR" && go build -trimpath -o "$BIN_DIR/postpilot-agent" ./cmd/postpilot-agent)
rm -rf "$SHARE_DIR/postpilot-publisher"
cp -R "$AGENT_DIR/hermes/postpilot-publisher" "$SHARE_DIR/postpilot-publisher"
chmod -R go-rwx "$SHARE_DIR/postpilot-publisher"
POSTPILOT_PLUGIN_DIR="$SHARE_DIR/postpilot-publisher" "$BIN_DIR/postpilot-agent" diagnostics || true
printf '%s\n' "Installed $BIN_DIR/postpilot-agent"
printf '%s\n' "Next: run '$BIN_DIR/postpilot-agent setup', then '$BIN_DIR/postpilot-agent install'."
