---
name: create-code-review
description: >-
  Run a READ-ONLY code-quality / architecture review and record the findings in spec/code-review/NN.slug.md. Use when
  the user wants an audit, a refactor-opportunity sweep, or a tech-debt report — "review the codebase", "audit the
  frontend for debt", "where can we refactor X", "do a code-quality pass". It scaffolds the next report with
  `pnpm spec:code-review "<title>"`, then fills it with evidence-backed findings (R001…) and candidate jobs. It does
  NOT modify code and does NOT implement — turn selected findings into a job with /create-refactor-job NN.
  All docs are written in English. Do NOT auto-commit.
---

# Create a code-review report (read-only audit → findings)

`spec/code-review/NN.slug.md` is a **read-only** code-quality and architecture review. It records refactor
opportunities only — no code changes here; implementation belongs in `spec/jobs/`. **Write the report in English.**

A good review is **two passes merged into one report** (this is what catches what per-job verification misses):

1. **Readiness / process pass** — *run the gates, don't just read them.* Toolchain parity, full-gate execution, CI
   parity, and enforcement-vs-documentation drift.
2. **Runtime-correctness pass** — a *multi-angle adversarial sweep* over the diff, then a verification pass that
   stamps each candidate CONFIRMED / PLAUSIBLE / REFUTED with a quoted line.

The bar is *evidence-backed*: every finding cites concrete `file:line`. Tests passing is **not** proof — a green test
can mask a bug (a fake that accidentally conforms); a missing test proves nothing.

## Steps

1. **Scope** — agree what's under review with the user: full repo, the frontend (`frontend/`), the backend
   (`backend/`), a module, or a diff (e.g. a finished phase: `git diff main...HEAD`, hand-written source only —
   exclude generated code under `*/gen/` and deletions). State it in the report's **Scope** section.

2. **Ground the judgment** — read the rules you'll judge against and record them in **Grounding** + **Architecture
   Baseline**: [spec/ARCHITECTURE.md](../../../spec/ARCHITECTURE.md) (§3 FSD layering on the FE, §2 backend
   domain-first package boundaries), the constitution ([00.overview](../../../spec/plan/00.overview.md) §3), SSOT
   discipline (plan/policy/tech), the config seam, and the relevant [PRD.md](../../../PRD.md) sections (the PRD owns
   the product-level decisions — §3 core design decisions, §6 stack).

3. **Pass A — Readiness / process (execute, don't assume).** This is the cluster per-job work always misses, so do it
   first and report it honestly in **Verification Notes**:
   - **Toolchain parity** — confirm the local runtime matches the declared `engines` (Node ≥20, pnpm 10.x) *before*
     trusting any result. A mismatch is itself a **P-finding**, not just a "blocked" note — it means the suites that
     catch the runtime bugs never ran. Look for an executable pin (`.node-version` / `.tool-versions` / `mise.toml`);
     its absence is a finding. Go and buf run in Docker — confirm the image tag matches `backend/go.mod`.
   - **Run the full gate** — actually execute `pnpm lint`, `pnpm --filter ./frontend lint:fsd`, the frontend
     typecheck/build (`pnpm build:web`), `pnpm --filter ./frontend test`, `pnpm lint:spec`, and the Go gate
     (`docker run --rm -v ${PWD}/backend:/app -w /app golang:1.26 sh -c "go vet ./... && go build ./... && go test ./..."`).
     A red baseline is a **P1 finding**, not a footnote. Record exactly what PASSED / FAILED / was BLOCKED and why.
   - **CI parity** — diff `.github/workflows/*` (`ci.yml`, `deploy-backend.yml`) against the root gate scripts in
     `package.json`. *Every* local gate must have a CI counterpart (frontend lint/typecheck/test, FSD lint, Go
     vet/build/test, spec hygiene, generated-freshness). Each hole is a finding: "CI can pass while X breaks."
   - **Enforcement vs documentation** — for every rule the SSOT *claims* is enforced (FSD boundaries via steiger,
     spec hygiene, config seam, the invariants), confirm an **executable guard exists** *and* a probe proves a
     violation actually fails. Documented-but-unenforced, or a guard whose allowlist/path has drifted from where
     files actually live, is a finding.

4. **Pass B — Runtime correctness (adversarial diff sweep).** Read + reason over the diff and its enclosing functions
   from independent angles, then verify each candidate (CONFIRMED / PLAUSIBLE / REFUTED, with a quoted line). Record
   **REFUTED** candidates too so they aren't re-raised. Apply these lenses:
   - **Async / lifecycle races** — events arriving during bootstrap/init, optimistic local state mutated before an
     `await` with no rollback on failure, expiry/refresh windows, provider dispose / re-init (StrictMode remount,
     re-init without closing the prior client), long-running generation jobs polled while the page unmounts.
   - **Degraded / partial-config & error branches** — missing-field *combinations* (one env var set, its pair unset),
     fatal-vs-non-fatal choices at boot, fall-through, unreachable `?? fallback` operands behind an earlier guard.
     Postpilot boots migrations in-process: a migration failure must kill the process, not degrade silently.
   - **SQLite single-writer discipline** — writes serialized on one connection, WAL assumptions, transactions that
     hold the writer across an LLM call, `busy_timeout` gaps.
   - **LLM provider seam** — the port in `internal/llm` leaking a vendor type upward, retries that duplicate a
     charged call, timeouts absent, prompt/response size unbounded, partial JSON parsed as success.
   - **Seam-without-consumer & masking tests** — set/get key-resolution asymmetry, a fake/test that accidentally
     conforms to the bug and stays green, scaffolding with no production caller, props/exports nothing passes.
   - **Config-seam drift** — hardcoded numeric tuning (TTLs, intervals, limits, model ids) living as a literal in a
     component or handler instead of flowing through `frontend/src/shared/config` /
     `backend/internal/platform/config`.
   - **Scaffold / MVP residue** — denylists, config, fixtures, or comments referencing a deleted concept or carried
     over from the scaffold (speculative config for types nothing emits, the health-check placeholder shape).
   - **Hot-path efficiency** — `O(n)` lookups where an `O(1)` map already exists, allocations on every render, image
     work on the main thread (PRD §6.2 puts image processing in the browser — check it doesn't block).
   - **Convention / hygiene** — render-phase external-store writes, test teardown not in `afterEach`/`try-finally`
     (leaks on assertion failure), comments recording plan/job/phase numbers or change history, comments that
     contradict the code.
   - **Latent footguns** — env-var name collisions across keys differing only by separator, flag gates that would
     swallow a future kill-switch.

5. **Scaffold + fill** — `pnpm spec:code-review "<title>"` → `spec/code-review/NN.slug.md` (English title; the scaffold
   slugifies it and stamps the date). Fill **Findings** as `R001`, `R002`, … each with priority (P1/P2/P3), area,
   evidence (`file:line`), why it matters, recommendation, and a suggested job split. Add **Cross-Cutting Themes**
   (name the *root cause*, e.g. "gate ran in a degraded env", "two copies diverged"), open **Questions / Tradeoffs**,
   and honest **Verification Notes** (what PASSED / FAILED / BLOCKED, plus the REFUTED candidates and the docs they
   were checked against).

6. **Candidate Jobs** — distill the findings into coherent implementation-job candidates (cluster related findings:
   e.g. all lifecycle fixes as one job, all cleanup as one). Each a sentence `/create-refactor-job` can turn into
   a job.

7. Report the report path + "Next: `/create-refactor-job NN` to turn selected findings into a job". Do NOT modify code
   or commit.

This is the read-only sibling of the diff reviewer `/code-review` — that one reviews the working tree inline; this one
produces a durable, numbered audit doc for planning refactor work.
