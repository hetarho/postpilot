---
name: create-plan
description: >-
  Author a NEW feature spec in spec/plan/ by interviewing the user, to the quality bar of the existing plan docs.
  Use when the user wants to plan/design/spec a new feature or capability — "plan a new feature", "create a plan/spec
  for X", "design how X should work", "write a planning doc". This skill interrogates the user (purpose · scope/non-goals
  · design · acceptance criteria), scaffolds the next sequential plan with `pnpm spec:plan "<title>"`, fills it, ALSO
  creates/updates the policy/tech docs it needs and enumerates the config values it introduces, and registers it in
  00.overview (status ⬜ planning). It does NOT implement — that's /create-plan-job then /implement-job.
  All docs are written in English. Do NOT auto-commit.
---

# Create a plan (interview → spec doc)

`plan/NN.*.md` is the **as-built SSOT** — the human-authored WHAT that makes implementation safe. Your job is to
turn a fuzzy request into a complete, reviewable plan. The product-level source of truth it must not contradict is
[PRD.md](../../../PRD.md); the structural one is [spec/ARCHITECTURE.md](../../../spec/ARCHITECTURE.md).
**Write the plan in English** (the PRD is Korean — that's fine; plans, changes, and jobs are English).

## Step 1 — Interview

Don't guess the spec — extract it. Ask the user a few focused questions at a time (use AskUserQuestion for real
forks) until you can write each section concretely:

- **Purpose** — why this feature; the user/product problem. Tie it to a PRD goal or scenario.
- **Scope / Non-goals** — what's in, and what's explicitly out (non-goals prevent scope creep). PRD §8 (범위 밖)
  is already a non-goal list — don't re-open it silently.
- **Design** — how it works: key decisions, the Connect RPC/proto contract, the SQLite data model and migration,
  job/state transitions, UI approach. Surface choices as questions when there's a real fork.
  Include **architecture placement** — which FSD layer/slice/segment (FE, `ARCHITECTURE.md` §3.1) or Go
  context/package (BE, §2) each new piece lands in — so the job and implementer inherit the placement instead of
  re-deriving it (or drifting into a flat layout). Invoke `/fe-architecture` · `/be-architecture` when unsure.
- **Acceptance Criteria** — the testable criteria that make it "true" (these become the job's acceptance criteria).
- **Policy / Config Impact** — which policy rules the feature sets, and which config values it introduces.

Honor the invariants **[I1]–[I7]** ([00.overview](../../../spec/plan/00.overview.md) §3, *The constitution*) — if the
request would break one (e.g. publishing directly to a platform, treating HTML as the canonical post, a second style
profile per account), flag it and resolve with the user before writing.

## Step 2 — Scaffold + fill

- `pnpm spec:plan "<title>"` → creates `spec/plan/NN.slug.md` (next number, stable ID — never reuse/renumber).
- **Pass an English title** so the file slug is English kebab-case. The scaffold slugifies `<title>` straight into
  the filename.
- Fill every section from the interview, concretely enough to re-implement on another client.
- Leave it as a plan (no checkboxes / no implementation) — implementation is a separate job.

## Step 3 — policy / tech / config it needs

A feature usually implies canonical rules or values. As part of creating the plan:

- **spec/policy/** — create or update the doc(s) the feature's rules belong to (e.g. `spec/policy/<x>.md`).
  policy describes *current truth*, so write the rule as "will be true once implemented" in the plan's **Policy /
  Config Impact** section now; the rule lands in policy/ when the job completes (see /implement-job Step 5).
  Don't pre-write unbuilt rules into policy as if shipped.
- **spec/tech/** — a doc per non-obvious technical decision the feature makes (a provider contract, an image
  pipeline, a retry policy), when the PRD doesn't already own it.
- **Config, not magic literals.** Any setting the feature introduces or changes (timeouts, caps, model ids,
  thresholds, defaults) is config: enumerate it in **Policy / Config Impact** and say who owns it. FE reads config
  through `frontend/src/shared/config` (Vite env), BE through `backend/internal/platform/config` (env → typed
  struct). A tuning number inlined in a component or handler is a placement bug. (Excluded: formulas, prompt text,
  and proto/DB schema — those live in code/proto/migrations.)

## Step 4 — Register in 00.overview

Add the plan to the **plan index** and the **progress board** with status **⬜ planning** (so other agents see it
exists, unbuilt). Add it to the dependency graph if it has prerequisites. Then report: the plan path, what
policy/tech you touched, and "Next: `/create-plan-job NN` to generate the implementation job". Do NOT implement
or commit.

For a CHANGE to shipped behavior, use /create-change instead (this skill is for new features).
