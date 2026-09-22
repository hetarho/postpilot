#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
AGENT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

# This former installer may only inspect retirement. It never replaces the
# installed binary or loads a KeepAlive job again.
printf '%s\n' "Automatic publishing has been retired. This command only inspects local cleanup."
(cd "$AGENT_DIR" && go run ./cmd/postpilot-agent retire)
printf '%s\n' "Review the paths above, then run the displayed retire --apply command from the reviewed retirement build."
