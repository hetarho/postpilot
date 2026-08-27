---
name: create-refactor-job
description: >-
  Generate the implementation job for selected findings from a code-review report. Use when the user wants to turn an
  audit into buildable work — "create the refactor job from review NN", "create-refactor-job NN", "build job from these
  findings". Reads spec/code-review/NN, scaffolds the next sequential spec/jobs/MM.slug.md with
  `pnpm spec:job refactor NN` (frontmatter type=refactor, source=code-review/NN, plan=none), then fills the
  Implementation Checklist + Affected files from the chosen findings. It does NOT implement — that's
  /implement-job MM. All docs are written in English. Do NOT auto-commit.
---

# Create an implementation job from a code-review report (refactor)

A refactor job turns selected findings from a `spec/code-review/NN.*.md` report into buildable steps. Its frontmatter
records `type: refactor`, `source: code-review/NN`, and `plan: none` (refactors aren't tied to one plan). **Write the
job in English.**

## Steps

1. **Target** — the code-review number NN (`spec/code-review/NN.*.md`). Confirm it's a filled report (findings R001…),
   not a bare template. If empty, send the user back to /create-code-review first.
2. **Pick the findings** — with the user, choose which findings (R0xx) and Candidate Jobs go into this job. A job
   should be one coherent unit of work; split unrelated findings into separate jobs rather than one mega-job. Pass a
   focused `pnpm spec:job refactor NN "<job title>"` so the slug names the slice.
3. **Scaffold** — `pnpm spec:job refactor NN ["<title>"]` → `spec/jobs/MM.slug.md` with `type: refactor`,
   `source: code-review/NN`, `plan: none`, `status: todo`. (A code-review report has no plan acceptance criteria to
   auto-copy, so you write the Acceptance Criteria from the chosen findings' "Recommendation"/"Why it matters".)
4. **Fill the checklists** — write **Acceptance Criteria** as the testable end-state for each chosen finding (behavior
   preserved, structure improved). Derive the ordered **Implementation Checklist** (`- [ ] T001 …`) from the findings'
   recommendations; `[P]` for independent files; flag `(gen)`/`(migrate)` where contracts or schema move.
   Refactors must not change behavior — make "no behavior change / no regression" an explicit acceptance item.
   Fill **Affected files** from the findings' evidence + a code grep.
5. **Register** — in the 00.overview progress board, note review NN → job MM.
6. Report the job path + "Next: `/implement-job MM`". Do NOT implement or commit.
