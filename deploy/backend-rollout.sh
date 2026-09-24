#!/bin/sh
# Run from /srv/postpilot-<env>; config snapshots are service-specific.
set -eu
exec python3 "$(dirname "$0")/media_rollout.py" api "$@"
