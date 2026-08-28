---
name: implement-job
description: >-
  Implement a job from spec/jobs/ end-to-end — the unified implementer for new builds, changes, AND refactors (the
  job's frontmatter `type: new|change|refactor` decides). Use when the user says "implement job NN", "/implement-job
  NN", "finish building this job", or to continue a scaffolded job. Claims the job in 00.overview (so concurrent agents
  don't collide), reads the source spec (plan, change, or code-review) + grounding, works the Implementation Checklist
  top-to-bottom (regen with `pnpm gen:proto` and add embedded migrations as contracts/schema move), verifies
  (acceptance criteria true + no regression for changes + build/lint/vet), code-reviews (/code-review), reflects the
  result into the SSOT (plan/policy/tech; archives the change doc if type=change), and marks the job done. All docs are
  written in English, but the final user-facing report is written in Korean and includes a reviewer-friendly
  explanation of the core implementation logic. Do NOT auto-commit.
---

# Implement a job (unified: new + change + refactor)

A **job** (`spec/jobs/NN.*.md`) is the buildable work doc. Its frontmatter tells you everything: `type`
(new|change|refactor), `source` (the plan, change, or code-review it implements), `plan` (the plan built/modified,
or `none` for refactors), `status`. You implement its **Implementation Checklist** and verify against its
**Acceptance Criteria**. **All docs you write or update are in English.** The final user-facing report is the exception:
write it in Korean, while keeping file paths, commands, identifiers, and generated labels readable as code/literals.
That final report must also explain the core logic clearly enough that the user can begin code review from the report,
not from a cold diff.

## Step 0 — Claim the job (concurrency)

Open `spec/jobs/NN.*.md`. Read its frontmatter. In [00.overview](../../../spec/plan/00.overview.md)'s **progress
board**, check the `plan:` isn't already 🟡 claimed by another in-flight job/agent. If free, set it 🟡 (job NN) and set
the job's frontmatter `status: doing`. This is how parallel agents avoid stepping on each other — claim before
building.

## Step 1 — Read the source + grounding

Read the `source` spec in full (`plan/NN` for new, `changes/NN` for change, `code-review/NN` for refactor — esp. its
acceptance criteria and scope/non-goals), its grounding ([spec/ARCHITECTURE.md](../../../spec/ARCHITECTURE.md) — the §
covering this job's surface, `spec/tech/*.md`, `spec/policy/**`, the relevant [PRD.md](../../../PRD.md) sections), and
the invariants **[I1]–[I7]** in 00.overview §3 (*The constitution*). For `type: change`, also read the modified `plan`
and note the existing acceptance criteria you must NOT regress. Grep the code the spec names — that's your blast
radius (it should match the job's Affected files).

**Architecture placement gate — invoke the placement skill for this job's surface *before* creating/moving files, and
re-run its self-audit at review time (Step 4):** FE (`frontend/src`) → **`/fe-architecture`**; Go under `backend/` →
**`/be-architecture`**. Each carries the layer/slice/segment (or context) decision procedure + a self-audit, backed by
`ARCHITECTURE.md` (§3 / §2). This is how structural rules enter the loop — skipping it is how an `app/` layer drifts
flat.

**Design-language gate — every FE surface (any file under `frontend/src` that renders UI) follows
[spec/tech/design-language.md](../../../spec/tech/design-language.md), always.** Read it before writing the first
`className`. Its four hard rules are not negotiable: (1) generic controls come from `shared/ui` — if the primitive you
need (input, dropdown, popover, …) does not exist, **add it to `shared/ui` first**, then use it; (2) only tokens
registered in Tailwind's `@theme` — the stock colour palette is removed, so `bg-gray-100` emits no CSS, and arbitrary
values (`p-[13px]`, `bg-[#…]`) are forbidden; (3) borders only where the guide lists an exception — depth and grouping
come from surface steps (`bg` → `surface` → `surface-raised`); (4) a card only when its contents must read as one unit
*and* apart from their neighbours. Its §9 checklist is part of the Step 4 review, and `pnpm lint:style` is part of the
Step 3 gate.

## Step 2 — Build the Implementation Checklist top-to-bottom

Implement tasks in order; flip `- [ ]` → `- [x]` in the job as you finish each. Match surrounding code style. Run the
matching regen when something ripples:

| You changed | Run |
|---|---|
| `proto/**/*.proto` (RPC contract) | `pnpm gen:proto` (or `pnpm gen`) → fix call sites |
| the SQLite schema | add a goose migration under the backend's embedded migrations dir; it runs at boot (PRD §6.3) — restart the API and confirm it applied |
| a frontend dependency / config | `pnpm --filter ./frontend …`, and put the value in `shared/config` |

**Config rule — no magic literals:** any newly needed or changed setting (timeouts, caps, model ids, thresholds,
defaults) does not sit inline in a component or handler. FE reads it through `frontend/src/shared/config` (Vite env);
BE reads it through `backend/internal/platform/config` (env → typed struct). If you hit an existing inline literal
while working, take the chance to move it into its owner. (Excluded: formulas, prompt text, and proto/DB schema —
those belong to code/proto/migrations.)

**Comment rule — only the "why" for a future reader (no narration / no history):** a comment explains only what the
*first person to read this code* can't get from the code itself. Judge by value vs cost — a good comment saves
thousands of tokens of code-tracing; a bad one just costs tokens and goes stale.
- **Worth keeping:** external constraints not visible in code (why the writer is a single connection; why this runs in
  the browser and not the server), non-obvious design intent, links to spec/invariants, traps and non-interference
  guarantees.
- **Drop it:** change history (`raised 0.4 -> 0.75`, `used to be 280` — git remembers), tautology that restates the
  code (`// bump time`), refactor/conversation narration (`artistic overhaul:`, `tuned this`), anything obvious from
  the current value. Past values and motivation go in the commit message, not the code.
- Write new comments to this bar, and **when you touch a file, clean up the "drop it" comments you pass, keeping only
  the "why"** (don't touch files outside the job's scope).

**Toolchain via Docker (this repo):** `go` and `buf` may not be on the host, so they run in Docker — the `pnpm`
scripts already do this. Ad-hoc Go:
`docker run --rm -v ${PWD}/backend:/app -w /app golang:1.26 sh -c "go build ./..."` (keep the tag in sync with
`backend/go.mod`). Generated code is committed — leave new `*/gen/` files for the user.

## Step 3 — Verify

1. Codegen/migration applied, no errors (not skipped).
2. FE specs → `pnpm build:web` + `pnpm lint` + `pnpm --filter ./frontend lint:fsd` + `pnpm lint:style` (0 errors), and
   `pnpm --filter ./frontend test` where tests exist. BE specs →
   `docker run --rm -v ${PWD}/backend:/app -w /app golang:1.26 sh -c "go vet ./... && go build ./... && go test ./..."`
   (the `&&` must run *inside* the container).
3. **Acceptance Criteria — the acceptance bar.** Go criterion by criterion; confirm each is **true in the running
   code** (a Connect call via `curl`/`buf curl`, `sqlite3 … .schema`, grep, a UI check). Tick each `- [ ]` in the
   Acceptance Criteria section.
4. **For `type: change` — no regression:** the existing `plan` acceptance criteria this change does not intend to
   alter still hold. **For `type: refactor` — behavior unchanged:** the refactor preserves observable behavior.
5. Constitution sanity — nothing in the diff breaks an invariant [I1]–[I7] (00.overview §3).
6. `pnpm lint:spec` — spec/workflow hygiene (doc links resolve, job status matches location, scaffolds substitute).

Fix and re-run any red check before reporting.

## Step 4 — Code review → refactor

1. **`/code-review` on the diff — always.** Apply findings; note rejections (in the job).
2. **`/codex:review` — for non-trivial logic** (skip small/mechanical; or if the user says "just /code-review").
   Real Codex engine, different model — strong on race/lifecycle + second-order bugs.
   - **Invoke `/codex:review --background`.** If asked "Wait / Run in background", choose background.
   - ⚠️ **Run Codex with `-m gpt-5.5`** (this box's ChatGPT-account CLI rejects every `*-codex` model, and the CLI
     default is one — a bare `codex exec` fails; 5.4 is the fallback). If `/codex:review` isn't invocable, run Codex
     directly as a background **Bash** task (NOT via `codex:rescue`):
     `codex exec --skip-git-repo-check --sandbox read-only -m gpt-5.5 - < prompt.txt > review.txt 2>&1`, then
     `TaskOutput(block=true)`.
   - ⚠️ **Never route codex through `codex:rescue` or any sub-agent** (double-background trap; a sub-agent can
     silently substitute a non-Codex review). If `/codex:status` shows codex unavailable, say so — don't pass off a
     non-Codex review as codex.
   - Run `/code-review` while codex runs (~25 min); **don't end the turn waiting** — block in-turn
     (`TaskOutput(block=true, timeout=600000)` ~3×), then merge (dedupe + severity-rank).
3. **Design pass (FE) — run the [design-language §9 checklist](../../../spec/tech/design-language.md) over the diff:**
   no hand-rolled controls, borders/cards only where the guide allows, one `solid primary` per view, both themes
   checked. Fix before moving on.
4. **Comment pass — sweep the diff's new/changed comments against the Step 2 comment rule.** Cut change history,
   tautology, and narration, keeping only the "why". Add a line where non-obvious code is *missing* its
   external-constraint/design-intent note. (If a separate pass is too much, fold "also check comment quality" into
   `/code-review`.)
5. Re-verify (Step 3) if the review changed code.

## Step 5 — Reflect into the SSOT, finish (and do NOT commit)

A job is done when the **docs are true again**, not just when code builds:

1. **`plan/NN.*.md`** — update to the new current reality (new fields/rules/scope/acceptance criteria). plan is
   as-built; no checkboxes. (For `type: refactor` with `plan: none`, update whatever plan(s) the touched code
   actually backs, if any.)
2. **`spec/policy/**` · `spec/tech/**`** — update any rule/contract the work set or changed (the owner doc). If the
   work changed a *product-level* decision the PRD owns, say so in the report and let the user decide about PRD.md —
   don't silently rewrite the PRD.
3. **Config owners** — a setting that was set or changed lives in `frontend/src/shared/config` /
   `backend/internal/platform/config` (and `.env.example` when it's a new env var).
4. **If `type: change`** — move the `source` `spec/changes/NN.*.md` → `spec/changes/archive/NN.*.md`.
5. **Close out** — job frontmatter `status: done`, all checkboxes ✅; 00.overview progress board for the `plan` → ✅
   (clear the 🟡 claim).
6. **Archive the job** — move this `spec/jobs/NN.*.md` → `spec/jobs/archive/NN.*.md`, so `jobs/` lists only
   todo/doing work. (Enforced: `pnpm lint:spec` fails if a `status: done` job is left in `spec/jobs/`.) The archived
   doc is a historical record — its relative links may go stale (no depth fix needed); only its frontmatter
   `source`/`plan` numbers must stay correct. Numbering stays safe: `pnpm spec:job` counts `archive/` too
   (monotonic), so the next job never reuses NN.

Report to the user in Korean, keeping commands, file paths, and identifiers verbatim. Be friendly and review-oriented:
summarize what changed, then explain the core logic and review path. The report must include:

- **Core logic:** the main runtime/data/control flow, where it enters, which modules own it, and why the implementation
  satisfies the spec.
- **Review guide:** the most important files or file groups to inspect first, plus what each proves.
- **Risk notes:** any subtle behavior, boundary, generated-code, config, migration, or verification caveat that a
  reviewer should keep in mind. If there are no special risks, say so plainly.
- **Verification evidence:** the exact commands that passed, not just "tested".

Use this shape, extending it when the job is broad:

```
✅ Job NN — <title> (<type>)  done
- source: <plan/NN | changes/NN | code-review/NN>  ·  plan: <plan/NN | none>
- implementation: Implementation Checklist T001–TNNN ✅
- codegen/migration: <result, or "n/a">
- core logic:
  - <entrypoint / flow 1: what happens and which files own it>
  - <flow 2, if relevant>
- review guide:
  - <file/group>: <what to review there>
  - <file/group>: <what acceptance criterion or invariant it proves>
- risk notes: <none, or concise caveats for reviewers>
- verification: <exact commands passed> · acceptance criteria <per-item ✅> (+ for change: no regression ✅)
- review: /code-review <applied N · rejected M + reason> (+ codex <merged> or "skipped")
- SSOT: plan <updated> · policy/tech <updated/none> · config <updated/none> (+ for change: changes/ archived)
- cleanup: job → jobs/archive/ (jobs/ now lists only todo/doing)
- commit: not done — changed files are staged for you. You commit with `type(planNN - scope): English title`; write
  the title in English and the body/comment in Korean.
```

**Don't run `git commit`.** The user commits.
