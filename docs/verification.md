# Task and pre-push verification

The policy is [ARCH-24 and ARCH-31](../spec/ssot/ARCH.md). Task completion and pushing have different verification scopes. Installed implementation/work-management skills use these stages when selecting their verification commands.

## Complete a task

Run the tests added or modified by the task and existing tests for plausible side effects. Select against the task's complete delta from its recorded starting revision, including intermediate commits, deleted files and working-tree changes. Record the commands and selection rationale in the task result.

Include FE consumers, shared harnesses and relevant user flows; include BE packages, reverse dependencies, adapters and composition wiring. Shared contracts, migrations, authentication and media changes require their corresponding integration regressions. If the impact cannot be bounded, expand to the affected full suite. No selected tests is not a passing behavioral check.

Vitest can select affected tests with `--changed` against the recorded task-start SHA, or `related --run` against source paths relative to `frontend/`. Import-based selection needs an explicit check for effects outside the import graph. Do not switch the baseline to the latest intermediate commit. Go tests select packages and, where the full package is expensive, explicit test names covering the changed behavior and its consumers.

Acceptance and relevant lint, formatting and build/type checks must also pass. Task completion, worker submission and individual task integration do not automatically run the pre-push gate. A final integration/qualification task still runs every check required by its own acceptance.

## Before push: full CI

Run once for the final push candidate, from the repository root, with the Node version in `.node-version`, Go from `backend/go.mod`, installed lockfile dependencies and Docker available. These are the checks in `.github/workflows/ci.yml`; changes to that workflow require updating this list.

```sh
(
set -eu
pnpm test:dev
pnpm gen:proto && git diff --exit-code -- backend/internal/gen frontend/src/shared/api/gen
pnpm gen:sql && git diff --exit-code -- backend
pnpm lint
pnpm lint:fsd
pnpm lint:retirement
pnpm lint:style:probe && pnpm lint:style
pnpm exec haeram-spec-creator check
pnpm --filter ./frontend test
pnpm build:web
python3 -m venv /tmp/postpilot-deploy-checks
/tmp/postpilot-deploy-checks/bin/pip install -r deploy/requirements-test.txt
/tmp/postpilot-deploy-checks/bin/python -m unittest discover -s deploy -p '*_test.py' -v
(
  cd backend &&
  test -z "$(gofmt -l .)" &&
  go vet ./... &&
  go build ./... &&
  go test -timeout 30m ./...
)
)
```

Each command must succeed before proceeding. Successful unchanged checks can be reused within this verification; changes to source, configuration, generated files or dependencies invalidate the affected checks. The evidence must describe the final candidate. This document is a runbook, not an installed Git hook or a command runner.

## Before a backend/media push: deployment and media parity

Check the actual push diff against `Deploy backend`'s `paths` filter, not just the last task's files. A backend deployment triggers `Verify media` for the successfully deployed revision. Changes to media/workflow definitions also require validating their affected gates, even when their path alone does not trigger deployment.

The general CI above does not build or execute the media images. Reproduce `.github/workflows/deploy-backend.yml`'s Compose/build checks and all three `.github/workflows/media-verify.yml` gates locally before a push that can run them. GPU activation has additional hardware gates in ARCH-37; CPU checks do not satisfy those gates.

### Compose validation

Use disposable copies and example settings so an existing development `.env` or `worker.env` remains intact. This renders configuration only. Run the following block from the repository root; stop if any copy or Compose command fails.

```sh
(
  set -eu
  verification_root="$PWD"
  verification_dir="$(mktemp -d /tmp/postpilot-compose-check.XXXXXX)"
  trap 'rm -rf "$verification_dir"' EXIT
  mkdir -p "$verification_dir/deploy/media"
  cp "$verification_root/docker-compose.prod.yml" \
    "$verification_root/docker-compose.media.colocated.yml" \
    "$verification_root/docker-compose.media.remote.yml" "$verification_dir/"
  cp "$verification_root/.env.production.example" "$verification_dir/.env"
  cp "$verification_root/deploy/media/worker.env.example" "$verification_dir/worker.env"
  cp "$verification_root/deploy/media/docker-compose.yml" "$verification_dir/deploy/media/"
  cp "$verification_root/deploy/media/worker.env.example" "$verification_dir/deploy/media/worker.env"
  cd "$verification_dir"
  export IMAGE_TAG=validation MEDIA_WORKER_IMAGE_TAG=validation API_UPSTREAM=postpilot-api-validation
  docker compose --env-file .env --env-file worker.env -f docker-compose.prod.yml -f docker-compose.media.colocated.yml config --quiet --no-env-resolution
  docker compose --env-file .env --env-file worker.env -f docker-compose.prod.yml -f docker-compose.media.remote.yml config --quiet --no-env-resolution
  docker compose --env-file worker.env -f deploy/media/docker-compose.yml config --quiet --no-env-resolution
)
```

### Deployable image builds

The workflows' `ubuntu-latest` runners use [GitHub's x64 Linux environment](https://docs.github.com/en/actions/reference/runners/github-hosted-runners#supported-runners-and-hardware-resources). Use Linux/amd64 to test the same target; emulated execution on another architecture does not establish native runner performance. These commands build local images without publishing or rolling out.

```sh
(
set -eu
verification_sha="$(git rev-parse HEAD)"
docker build --platform linux/amd64 --target deployable --build-arg SOURCE_REVISION="$verification_sha" -t postpilot:verify-deploy-api -f backend/Dockerfile .
docker build --platform linux/amd64 --target media-worker-deployable --build-arg SOURCE_REVISION="$verification_sha" -t postpilot:verify-deploy-worker -f backend/Dockerfile .
)
```

### All three media gates

Use the workflow's exact image targets and execution budgets. The release runner's defaults match the workflow: both layouts, API 256 MiB/1 CPU, worker 512 MiB/1 CPU, a 1 GiB stated execution envelope and a 1500-second deadline. It creates isolated fixture storage, networks, containers and volumes and cleans its own resources.

```sh
(
set -eu
docker build --platform linux/amd64 --target production -t postpilot:gate -f backend/Dockerfile .
docker build --platform linux/amd64 --target media-worker-smoke -t postpilot-worker-smoke:ci -f backend/Dockerfile .
docker run --rm --cpus=2 --memory=1g postpilot-worker-smoke:ci
docker build --platform linux/amd64 --target media-storage -t postpilot:gate-media-storage -f deploy/media/fixture.Dockerfile .
docker build --platform linux/amd64 --target media-release-api -t postpilot:gate-media-release -f backend/Dockerfile .
docker build --platform linux/amd64 --target media-release-worker -t postpilot:gate-media-release-worker -f backend/Dockerfile .
python3 scripts/media-release.py --skip-build --skip-storage-build
)
```

The explicit builds keep fixture execution from rebuilding stale or differently targeted images. A cached build is valid only when Docker has evaluated the current candidate's inputs; merely finding an old image tag is not verification. Do not raise a budget, disable a gate or reuse stale images to count the default workflow as passed.

## After push: diagnose the actual workflow

Inspect `CI`, `Deploy backend` and `Verify media` when triggered, for the pushed SHA. `Verify media` checks the deployed workflow's `head_sha`; a result for another revision is not evidence for this push. A cancelled superseded run is distinct from a failed current run.

For a failure, record the workflow/run URL, SHA, job, step and first meaningful error. Classify the observed error as an assertion/application failure, image/build failure, OOM/resource limit, timeout, registry/network/authentication error, or production rollout/health/CORS failure. Do not infer an intermittent test from the red workflow name alone, suppress a failing media job or use indiscriminate retries as a fix.

A local pass covers the reproducible build/test checks. GHCR publication, SSH, production configuration, host capacity, deployment health and external CORS still require the remote result. Diagnose that failed boundary and run its relevant reproduction after a fix, rather than restarting all task suites without evidence.
