---
name: create-plan-job
description: >-
  Generate the implementation job for an existing NEW plan. Use when the user wants to turn a plan into a buildable
  work doc — "create the job for plan NN", "create-plan-job NN", "prep plan NN for implementation". Reads spec/plan/NN,
  scaffolds the next sequential spec/jobs/MM.slug.md with `pnpm spec:job plan NN` (frontmatter type=new, source/plan=
  plan/NN; the plan's acceptance criteria are auto-copied into the job's Acceptance Criteria), then fills the
  Implementation Checklist + Affected files from the plan's Design. It does NOT implement — that's
  /implement-job MM. All docs are written in English. Do NOT auto-commit.
---

# Create an implementation job from a plan (new)

A **job** (`spec/jobs/MM.slug.md`) is the buildable work doc: two checklists — **Acceptance Criteria** (from the
plan, the WHAT to verify) and **Implementation Checklist** (the STEPS to do). This skill creates it from a plan; it
doesn't build it. **Write the job in English.**

## Steps

1. **Target** — the plan number NN (`spec/plan/NN.*.md`). Confirm it exists and is the right one.
2. **Scaffold** — `pnpm spec:job plan NN` → creates `spec/jobs/MM.slug.md` (next job number, independent sequence)
   with frontmatter `type: new`, `source: plan/NN`, `plan: plan/NN`, `status: todo`, and the plan's **acceptance
   criteria auto-copied** into the Acceptance Criteria section. The scaffold takes the title from the plan, so the
   English slug carries over automatically.
3. **Fill the Implementation Checklist** — derive ordered `- [ ] T001 …` from the plan's Design (the HOW). `[P]` for
   parallel (different files, no dep); flag `(gen)` where the proto contract moves and `(migrate)` where the SQLite
   schema moves (postpilot embeds goose migrations and runs them at boot — a migration task means a new file under
   the backend's migrations dir, not a separate migrate step). **Any config value the plan calls for → a task that
   puts it in its owner (`frontend/src/shared/config` or `backend/internal/platform/config`), not inline in a
   component or handler.** Fill **Affected files** (blast radius from the plan + a code grep), noting for each its
   **target placement** — FE layer/slice/segment (`ARCHITECTURE.md` §3.1) or BE context/package (§2), so
   `/implement-job` places it right — and the grounding (`ARCHITECTURE.md` §3/§2 + the PRD/policy/tech it depends on).
   Don't pad the Acceptance Criteria — those came from the plan; refine only if the plan's wording isn't checkable.
4. **Register** — in the 00.overview progress board, note that plan NN now has job MM (still ⬜/🟡 until implemented).
5. Report the job path + "Next: `/implement-job MM`". Do NOT implement or commit.

For a CHANGE to shipped behavior, use /create-change → /create-change-job instead (this skill is
for new plans).
