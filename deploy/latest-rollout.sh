#!/bin/sh
# Run only after acquiring the GitHub rollout concurrency lock. Compare workflow
# runs, not branch HEAD: a later docs-only commit need not request a deployment.
set -eu
latest_run_id="$(gh api --method GET \
  "repos/${GITHUB_REPOSITORY}/actions/workflows/deploy-backend.yml/runs" \
  -f "branch=${GITHUB_REF_NAME}" -f per_page=1 --jq '.workflow_runs[0].id')"
case "$latest_run_id" in
  ''|*[!0-9]*) echo 'Cannot determine the latest deployment run' >&2; exit 1 ;;
esac
if [ "$latest_run_id" = "$GITHUB_RUN_ID" ]; then
  echo 'deploy=true' >> "$GITHUB_OUTPUT"
else
  echo 'deploy=false' >> "$GITHUB_OUTPUT"
  echo 'A newer deployment run exists; this run will not change the server.' >> "$GITHUB_STEP_SUMMARY"
fi
