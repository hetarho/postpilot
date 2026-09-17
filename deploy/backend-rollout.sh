#!/bin/sh
# Run from /srv/postpilot-<env> with IMAGE_TAG and API_ORIGIN set.
set -eu

# The caller selects the stack directory. Keep the incoming SHA out of Compose's
# process environment: exported variables override .env even during rollback.
TARGET_IMAGE_TAG="${IMAGE_TAG:?IMAGE_TAG is required}"
unset IMAGE_TAG

compose() {
  docker compose -f docker-compose.prod.yml "$@"
}

# Read values through Compose itself, so quoted empties and inline comments
# mean here exactly what they will mean inside the container.
resolved_runtime_env="$(compose config --environment)"
runtime_env_value() {
  printf '%s\n' "$resolved_runtime_env" | sed -n "s/^$1=//p" | tail -n 1
}
require_nonempty_env() {
  if [ -z "$(runtime_env_value "$1")" ]; then
    echo "required runtime env $1 is missing or empty" >&2
    exit 1
  fi
}

# Fail BEFORE touching the running stack when the stack .env is incomplete.
# API_UPSTREAM wrong = Caddy cannot find this api; CORS_ORIGIN wrong = every
# browser call fails auth (the session cookie needs an exact-origin CORS
# allow with credentials — PRD F-1).
require_nonempty_env API_UPSTREAM
require_nonempty_env CORS_ORIGIN

if [ -z "${API_ORIGIN:-}" ]; then
  echo "repo variable API_ORIGIN is required for the health gate" >&2
  exit 1
fi

set_image_tag() {
  if grep -q '^IMAGE_TAG=' .env; then
    sed "s|^IMAGE_TAG=.*|IMAGE_TAG=$1|" .env > "$ENV_BACKUP.next"
    cat "$ENV_BACKUP.next" > .env
    rm -f "$ENV_BACKUP.next"
  else
    echo "IMAGE_TAG=$1" >> .env
  fi
}

# Keep the whole untracked env file, so a rollback restores the previous image
# selection without disturbing any other runtime secret.
umask 077
ENV_BACKUP=.env.deploy-backup
cp -p .env "$ENV_BACKUP"
trap 'rm -f "$ENV_BACKUP" "$ENV_BACKUP.next"' EXIT

PREVIOUS_TAG="$(runtime_env_value IMAGE_TAG)"

health_gate() {
  curl --retry 15 --retry-all-errors --retry-delay 2 \
    --connect-timeout 5 --max-time 10 \
    --fail --silent --show-error \
    --output /dev/null "${API_ORIGIN}/health"
}

rollback() {
  echo "$1; rolling back to ${PREVIOUS_TAG}" >&2
  # Capture startup/migration errors before replacing the failed container.
  compose ps -a >&2 || true
  compose logs --no-color --tail 80 api >&2 || true
  cp -p "$ENV_BACKUP" .env
  if [ -n "$PREVIOUS_TAG" ]; then
    if compose up -d api && health_gate; then
      echo "rollback healthy: ${PREVIOUS_TAG}" >&2
    else
      echo "rollback failed; API needs recovery" >&2
      compose logs --no-color --tail 80 api >&2 || true
    fi
  else
    echo "no previous image tag; API needs recovery" >&2
  fi
  exit 1
}

# 1) Pull the new image BEFORE stopping anything. A pull failure must not cost
#    downtime.
set_image_tag "$TARGET_IMAGE_TAG"
if ! compose pull api; then
  cp -p "$ENV_BACKUP" .env
  exit 1
fi

# 2) Swap. SQLite is a single-writer file in this stack's volume, so the old
#    container must be gone before the new one opens it — `up -d` replaces the
#    container rather than running both.
#    --remove-orphans drops containers for services deleted from the compose
#    file instead of leaving them running.
if ! compose up -d --remove-orphans api; then
  rollback "new image failed to start"
fi

# 3) Health gate. The new binary runs its embedded migrations at startup, so
#    this is also the migration gate: a bad migration means the process never
#    serves /health. Retry generously — the container has to start, migrate,
#    and Caddy has to notice the new upstream.
if ! health_gate; then
  rollback "new image failed the health gate"
fi

# 4) Reclaim the images this deploy replaced. Every rollout pulls a NEW TAG, so
#    a bare prune removes only dangling layers and finds nothing: the tagged
#    images accumulated until the box had no room left for a render workspace.
#    -a takes the tagged ones too, and the window keeps the last day of images
#    so a rollback still has a local one to start.
docker image prune -af --filter "until=24h"
