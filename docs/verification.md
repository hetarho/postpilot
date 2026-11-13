# Task, batch and CI verification

The project commands and qualification constraints are [ARCH-24 and ARCH-31](../spec/ssot/ARCH.md). Execution, workspace ownership, review, commits and publication follow the installed Gusta skills and `spec/SETTINGS.md`.

## Complete a task

Reuse existing behavioral coverage and add or change tests only for a real gap. Task acceptance names affected unit tests and applicable fast static checks. Run these once just before commit through `vibe check --tier task`, reusing still-valid evidence. Do not count an empty selection as behavioral verification.

Assess the complete task delta against its recorded starting revision, including intermediate commits, deleted files and working-tree changes. Include FE consumers and shared harnesses, and BE reverse dependencies, adapters and composition wiring. Import-based selection is an aid; it does not prove complete impact coverage.

Vitest can select related unit tests with `--changed` against the task-start SHA or `related --run` against source paths relative to `frontend/`. Select Go packages and test cases for the changed isolated behavior. Inspect actual dependencies before classifying tests: SQLite/process/transport/browser integration tests stay at the coordinator's batch gate, regardless of their filename.

Use applicable static checks expected under a minute. Frontend commands are `pnpm --filter ./frontend lint`, `pnpm --filter ./frontend format:check` and `pnpm --filter ./frontend exec tsc --noEmit -p tsconfig.app.json`; select the affected fast formatter/type checks for other paths. The batch gate still runs the complete checks listed below.

Maintain the matching integration/e2e tests in the implementation, and hand their exact commands, test paths, input revision and any extra consumer coverage to the coordinator in result limits. An unbounded impact expands the batch selection to the affected full suite. Record the unit/static commands, actual evidence, selection rationale and unrun coverage in the task result. Task completion covers implementation, affected-unit verification and the handoff; it does not certify the later batch gate.

## Coordinator-owned final gate

The initiating Gusta coordinator runs builds, integration/e2e and full suites once on the synchronized committed implementation candidate before landing or whole-work completion, including when committing only. Implementation/review units hand off these checks; a unit or session ending does not start another full gate. Real supplier, human, hardware and live-deployment qualification remains an explicit activation or delivery condition.

Apply Gusta's cache, process cleanup and cumulative gate limits. Reuse each still-valid successful check; after a targeted repair run failed, affected or unrun checks. Reuse depends on actual consumed inputs, command, environment and scope, including transitive scripts/configuration/dependencies. A docs-only merge is not sufficient proof by itself. Optional named verification plans may store separate check receipts; do not infer a narrower input set from a script name. Stop writes to the candidate while checking, and invalidate evidence when its inputs change.

## Batch: full CI-equivalent coverage

The coordinator runs this coverage for the final implementation candidate, from the repository root, with the Node version in `.node-version`, Go from `backend/go.mod`, installed lockfile dependencies and Docker available. These are the checks in `.github/workflows/ci.yml`; changes to that workflow require updating this list.

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
npx -y @haeram/gusta@$(node -p 'require("./.gusta-lock.json").packageVersion') check
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

## Conditional batch: deployment and media parity

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
