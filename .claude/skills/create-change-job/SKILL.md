---
name: create-change-job
description: >-
  Generate the implementation job for an existing CHANGE proposal. Use when the user wants to turn a change into a
  buildable work doc — "create the job for change NN", "create-change-job NN", "prep change NN for implementation".
  Reads spec/changes/NN, scaffolds the next sequential spec/jobs/MM.slug.md with `pnpm spec:job change NN` (frontmatter
  type=change, source=changes/NN, plan=the modified plan from the change's frontmatter; the change's acceptance
  criteria are auto-copied into the Acceptance Criteria), then fills the delta Implementation Checklist + Affected
  files. It does NOT implement — that's /implement-job MM. All docs are written in English. Do NOT auto-commit.
---

# Create an implementation job from a change

Same shape as /create-plan-job, but the source is a change proposal, so the job is a **delta** and carries
a regression boundary. The job frontmatter records `type: change` and the `plan:` it modifies. **Write the job in
English.**

## Steps

1. **Target** — the change number NN (`spec/changes/NN.*.md`). Confirm its WHAT (as-is → to-be · acceptance criteria)
   is filled (it should be, from /create-change). If it's still a bare template, send the user back to
   /create-change first.
2. **Scaffold** — `pnpm spec:job change NN` → `spec/jobs/MM.slug.md` with `type: change`, `source: changes/NN`,
   `plan: <modified plan>` (read from the change's frontmatter), and the change's **acceptance criteria auto-copied**
   into the Acceptance Criteria section. The scaffold carries the English title/slug over from the change.
3. **Fill the Implementation Checklist** — ordered `- [ ] T001 …` for the **delta only** (as-is → to-be difference),
   with `[P]`/`(gen)`/`(migrate)` flags. **Config values the change adds or tweaks → a task that puts them in
   `frontend/src/shared/config` or `backend/internal/platform/config`, never inline.** Fill **Affected files**
   by grepping the as-is code (the blast radius). The job's DoD already carries "no regression of the existing plan's
   acceptance criteria" — keep it.
4. **Register** — in the 00.overview progress board, link the change → job MM.
5. Report the job path + "Next: `/implement-job MM`". Do NOT implement or commit.
