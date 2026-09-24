#!/bin/sh
# Run from the standalone directory containing docker-compose.yml + worker.env.
set -eu
exec python3 "$(dirname "$0")/../media_rollout.py" worker "$@"
