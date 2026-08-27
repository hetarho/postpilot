---
name: create-change
description: >-
  Author a CHANGE proposal in spec/changes/ by interviewing the user — for modifying an ALREADY-IMPLEMENTED plan.
  Use when the user wants to change/extend/refactor shipped behavior — "change how X works", "modify the existing
  NN behavior", "extend/refactor this feature", "let's manage X in the DB instead". It interrogates the user for the
  delta (as-is → to-be · scope/non-goals · acceptance criteria) — NOT a one-line prompt — scaffolds the next
  sequential change with `pnpm spec:change <planNN> "<title>"`, fills it, and notes affected policy/tech/config.
  It does NOT implement — that's /create-change-job then /implement-job. All docs are written in
  English. Do NOT auto-commit.
---

# Create a change proposal (interview → WHAT delta)

`spec/changes/NN.slug.md` is the **human-authored WHAT of a change** to an existing plan — the analogue of a plan,
but a delta. Authoring it by interview (not a one-liner) is the safety gate: the agent must not invent scope.
**Write the change in English.**

## Steps

1. **Pin the target plan** — which `spec/plan/NN.*.md` does this change modify? If unclear, find the plan whose
   current behavior the user is describing and confirm in one line.
2. **Interview the delta** — ask a few focused questions at a time (AskUserQuestion for real forks) until you can
   write concretely:
   - **As-is** — how the touched part works now (from plan/NN + a quick code grep).
   - **To-be** — the desired behavior/contract after the change.
   - **Scope / Non-goals (regression boundary)** — what changes, and what must NOT (the regression boundary).
   - **Acceptance Criteria** — testable criteria for the change (these become the job's acceptance criteria).
   Honor the invariants [I1]–[I7] ([00.overview](../../../spec/plan/00.overview.md) §3) — a change never breaks one.
3. **Scaffold + fill** — `pnpm spec:change <planNN> "<title>"` → `spec/changes/NN.slug.md` (frontmatter
   `change`/`plan`/`status`/`title`). **Pass an English title** so the file slug is English kebab-case; the scaffold
   slugifies `<title>` straight into the filename. Fill the sections from the interview. Note affected
   `spec/policy/**`/`spec/tech/**`; **any config value the change adds or tweaks goes to its owner**
   (`frontend/src/shared/config` · `backend/internal/platform/config`), never inline — list it so the job does that.
4. **Register** — in the 00.overview progress board, note the in-flight change against plan NN.
5. Report the change path + "Next: `/create-change-job NN`". Do NOT implement or commit.

For a brand-new (unbuilt) feature, use /create-plan instead.
