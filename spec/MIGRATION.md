# Spec-System Migration — plan/changes/jobs → haeram-spec-creator (STATE · ssot · tasks)

status: **executing** — started 2026-09-05 on the owner's go-ahead, with the recommended D1–D5 defaults; cfg.level = mid
written: 2026-09-05
depends on: haeram-spec-creator ^0.1.1 (installed, 6 skills synced to `.claude/skills` + `.codex/skills`)

The owner's decision: postpilot moves entirely to the haeram-spec-creator workflow
(`create-architecture` · `create-ssot` · `update-ssot` · `create-task` · `implement-task` ·
`create-narrative`). The 10 legacy project skills are **retired and deleted**, and the legacy
spec tree (`plan/` `changes/` `jobs/` `code-review/` `00.overview.md` `ARCHITECTURE.md`
`policy/` `tech/`) is converted into the new shape (`spec/STATE.md` · `spec/FORMAT.md` ·
`spec/ssot/*.md` · `spec/tasks/T###.md`).

## 1. Goal & non-goals

**Goal** — after migration, any session can be driven with only the new skills:
"기능 기획하자" → create-ssot, "task로 쪼개줘" → create-task, "다음 태스크 구현해줘" →
implement-task. `spec/STATE.md` is the single control tower; the rev/tasked gap is the
single source of "what is planned but not yet implemented".

**Non-goals**
- No product behavior changes. No code changes except removing spec tooling
  (`scripts/new-*.mjs`, `spec:*` package scripts) and the skill folders.
- No rewrite of history: git history remains the archive of record; the legacy docs are
  converted for *current truth*, not transcribed.
- No upgrade of haeram-spec-creator itself (stays ^0.1.1; upgrades are a normal dep bump +
  `haeram-spec-creator install` later).

## 2. Current inventory (what migrates)

| Legacy asset | Count | Destination |
| --- | --- | --- |
| `spec/plan/01–18` (+ `00.overview.md`) | 18 + 1 | Current-truth content → domain SSOTs (§5); overview retired — STATE.md replaces it |
| `spec/policy/*.md` | 17 | Merged into their domain SSOTs (policies are product decisions — SSOT body) |
| `spec/tech/*.md` | 11 | Domain SSOT tech notes, or `ssot/ARCH.md` when cross-cutting |
| `spec/ARCHITECTURE.md` | 1 | `spec/ssot/ARCH.md` (via create-architecture) |
| `spec/changes/` active: 09, 28, 29, 30 | 4 | Converted to SSOT deltas (rev+1 + chg) → tasks; archive (26 docs) stays in legacy |
| `spec/jobs/` active: 25 | 1 | See §6 dispositions; archive (54 docs, incl. job 56) stays in legacy |
| `spec/code-review/01` | 1 | Legacy archive; any still-open findings become tasks |
| Skills ×10 (`.claude/skills` + `.codex/skills`, git-tracked) | 20 dirs | **Deleted.** Knowledge from be-architecture · fe-architecture · library-setup is absorbed first (§4 Phase 1) |
| `scripts/new-plan.mjs` `new-change.mjs` `new-job.mjs` `new-code-review.mjs` + `scripts/templates/*` + `spec:*` scripts | 4 + 4 + 4 | Deleted (the new system scaffolds via its own skills) |
| `scripts/check-spec-hygiene.mjs` + `lint:spec` / `lint:spec:probe` (package.json + two CI steps) | 1 + 2 | Deleted — it validates job location and scaffold templates that no longer exist, and ~1.3k inbound links into the legacy tree; CI gains `pnpm exec haeram-spec-creator check` instead (D5) |
| Code comments citing `spec/plan|policy|tech|changes|jobs/...` or `spec/ARCHITECTURE.md` (~60 files incl. proto + .sql sources and their generated output) | ~60 | Rewritten to `spec/legacy/...`; proto/sql edits followed by `pnpm gen:proto` + `pnpm gen:sql` so generated files stay in sync (CI diff gate) |
| `PRD.md` · `DEPLOY.md` · `README.md` (root docs) | 3 | Kept at root. `ssot/` supersedes PRD.md for decisions (stated in ARCH); README's authority pointer moves to spec/STATE.md; DEPLOY.md stays the ops runbook |

Installed-but-untracked today: the 6 package skill folders and `.haeram-spec-creator-lock.json`
(→ D5).

## 3. Target state

```text
spec/
├── STATE.md        # control tower: cfg · SSOT rev/tasked board · task board · next · log
├── FORMAT.md       # notation rules for all spec docs
├── MIGRATION.md    # this file — delete when Phase 4 closes
├── ssot/
│   ├── ARCH.md     # stack, layout rules (FSD, domain-first Go), conventions, verify commands
│   └── <DOMAIN>.md # ~13 domain SSOTs (§5), decisions [o]/[?]/[x] + rationale, rev-tracked
├── tasks/          # T###.md — fresh numbering, legacy job/T ids referenced in body for traceability
└── legacy/         # (D1) frozen old tree, read-only, deleted after settling period
```

Skills: only the 6 from haeram-spec-creator, synced by `pnpm exec haeram-spec-creator
install` / verified by `… check`.

## 4. Phases

Each phase is a separate session (or more), committed straight to main per phase with
explicitly staged paths (parallel sessions share this tree).

### Phase 0 — Land or freeze in-flight work *(old system, prerequisite)*

1. **Job 56** (change 30, template screen rework) — verified 2026-09-05: already `status: done`
   and archived at HEAD 376c56c together with change 30 (confirmed by the session that built it);
   the overview row still read 🟡 and was simply stale. No Phase 0 work needed.
2. Verify dispositions in §6 for job 25 and changes 09/28/29 (their exact state may have
   moved by execution time — re-check `00.overview.md` first).
3. Freeze the old system: banner at the top of `00.overview.md` — *"FROZEN — no new
   plans/changes/jobs. See spec/MIGRATION.md."* Stop using `pnpm spec:*` from this point.

**Exit criteria**: job 56 landed; overview shows no 🟡 rows except items §6 marks "convert".

### Phase 1 — Initialize the new system

1. Run **create-architecture** in a dedicated session: user-level calibration (`cfg`),
   `spec/STATE.md`, `spec/FORMAT.md`, `spec/ssot/ARCH.md`.
2. Seed ARCH.md from (do not re-interview what is already decided — cite and confirm):
   - `spec/ARCHITECTURE.md` — all sections, especially §2 (domain-first Go: composition
     root → context behavior → domain, dependency rule, proto/sql anti-corruption boundary)
     and §3 (Feature-Sliced Design: layer → slice → segment, app-layer segment rule);
   - the **be-architecture** and **fe-architecture** skills — their decision procedures and
     self-audit checklists move into ARCH.md so implement-task can enforce placement;
   - the **library-setup** skill — its "current docs first, latest version, verify what
     landed" discipline becomes an ARCH.md convention for dependency work;
   - verify gates from the old implement-job: build/lint/vet + FE tests, `pnpm gen:proto`
     on contract moves, embedded goose migrations on schema moves, review pass before done.
3. Coexistence rule for the transition window: old tree is read-only reference; STATE.md is
   the only claim board for new work.

**Exit criteria**: STATE.md + FORMAT.md + ssot/ARCH.md exist; ARCH.md demonstrably covers
the three absorbed skills (checklist present) — this gates deleting them in Phase 3.

### Phase 2 — Convert content into domain SSOTs and tasks *(the bulk)*

1. Confirm the domain map (§5, decision D3) — then per domain, run **create-ssot** seeded
   with the legacy sources: extract *current truth* (shipped decisions `[o]` with rationale,
   open questions `[?]`, rejected paths `[x]`), not history. Shipped domains register in
   STATE with `tasked == rev` (nothing pending).
2. Conversion QA checklist, per domain — each item ticked before the domain counts done:
   - acceptance criteria of the source plan represented as decisions;
   - config values the plan introduced are listed;
   - regression-pinned defects and non-goals carried (e.g. TEMPLATE: the four grammar
     defects — U+FEFF/U+0085 disagreement, slot-order reversal, two-token block, quoted
     slot label — and "원문 mode removed" stay recorded);
   - destructive/irreversible decisions kept with their justification (e.g. migration 0022).
3. Open work (§6): encode as SSOT content with STATE pending (rev > tasked), then run
   **create-task** to decompose into `T###`. Carry the legacy acceptance criteria verbatim;
   reference legacy ids (job 25, T017–T021, change 09/28/29) in task bodies.
4. Domains are independent files — they can be converted in parallel sessions. STATE.md is
   the one shared file: claim a domain in STATE before converting it, stage explicit paths.

**Exit criteria**: every §5 domain has an SSOT; STATE board lists all of them; all §6
"convert" items exist as tasks; QA checklist ticked per domain.

### Phase 3 — Retire the old system

Gated on Phase 2 exit. Order matters — references first, then files:

1. Reference sweep: `grep -rn` for old skill names, `spec/plan/`, `00.overview`,
   `spec:job|spec:plan|spec:change|spec:code-review`, `ARCHITECTURE.md` across the repo
   (README, CLAUDE/agents docs if any, CI, `.claude/` `.codex/` settings, scripts). Fix or
   point at legacy/.
2. Move `spec/plan/ changes/ jobs/ code-review/ ARCHITECTURE.md` → `spec/legacy/` (D1).
   `policy/` and `tech/` move too — their content now lives in the SSOTs.
3. `git rm -r` the 10 legacy skills from **both** `.claude/skills/` and `.codex/skills/`
   (keep the 6 package-managed folders — `.haeram-spec-creator-lock.json` knows which files
   are the package's; never hand-edit those).
4. Remove `spec:*` scripts from `package.json` and delete `scripts/new-*.mjs`.
5. D5: commit the 6 installed skill folders + lock file so clones don't need a manual
   install step.

### Phase 4 — Verify & close

- `pnpm exec haeram-spec-creator check` → "정상: 6개 (claude, codex)".
- Fresh session lists exactly the 6 new skills and none of the old 10.
- Smoke the loop end-to-end: "다음 태스크 구현해줘" claims the right `T###` (doing) →
  implements → verify gates → done reflected in task file + STATE; "기획 바꾸자" on a small
  policy bumps rev + chg + pending correctly.
- Reference sweep grep comes back clean.
- Optional but recommended: **create-narrative** → `spec/NARRATIVE.md`; reading it is the
  final conversion-loss check (R1).
- Delete `spec/MIGRATION.md`; schedule legacy/ deletion (D1) after the settling period.

## 5. Domain SSOT map — draft (D3)

The 17 policy files already partition the product; SSOTs merge each policy with its plans
and tech notes. Draft — finalize at Phase 2 kickoff:

| SSOT | Legacy sources |
| --- | --- |
| ARCH | ARCHITECTURE.md · be/fe-architecture + library-setup skills · design-language(?) |
| AUTH | policy/auth · plan 01 |
| QUOTA | policy/plans · plan 17 · tech/usage-quota-and-plan-gating |
| POST | policy/posts · policy/uploads · plan 02 · tech/draft-autosave |
| VOICE | policy/voice · plans 03, 10 · tech/voice-personalization-learning · multi-voice-partitioning |
| GEN | policy/generation · policy/jobs · policy/revision · plans 05, 06, 07 |
| MODEL | policy/providers · policy/model-experiments · plans 04, 09, 18 · tech/openrouter-catalog · model-experiment-methodology |
| TEMPLATE | policy/templates · plan 11 · tech/post-template-grammar |
| GUIDE | policy/guidelines · plan 16 |
| EXPORT | policy/export · plan 08 |
| PUBLISH | policy/publishing · plan 12 · tech/paired-local-publishing-agent |
| LANG | policy/languages · plan 13 · tech/ui-localization-and-error-contract · content-language-and-voice-projection |
| THEME | policy/themes · plan 14 · tech/design-language(?) |
| MARKETING | policy/public-marketing · plan 15 |

Open granularity questions `[?]`: fold REVISION into GEN or split; design-language under
ARCH vs THEME; whether uploads deserves its own SSOT.

## 6. Open-work disposition (as of 2026-09-05 — re-verify in Phase 0)

| Item | State today | Disposition |
| --- | --- | --- |
| Job 56 / change 30 (template screen) | done + archived at HEAD (overview row was stale) | **Nothing to do**; lands in TEMPLATE SSOT as shipped truth |
| Job 25 (plan 12 publisher, T017–T021) | 🟡 doing — 26 checklist items done, 21 open (live Naver verification pending) | **Convert** (D2): PUBLISH SSOT carries pending rev; T017–T021 re-cut as new T### tasks |
| Change 09 (persistent browser sessions) | planning | **Convert**: PUBLISH SSOT delta + tasks |
| Change 28 (credit holds from stage budget) | active doc | **Convert**: QUOTA/GEN SSOT delta + tasks (confirm not already implemented) |
| Change 29 (write-budget reasoning headroom) | active doc | **Convert**: GEN/MODEL SSOT delta + tasks (confirm not already implemented) |
| Plan 15 (marketing page) | ⬜ planning, unbuilt | **Convert whole**: MARKETING SSOT with everything pending |
| code-review/01 open findings | to audit | Any still-relevant finding → task; else legacy |

## 7. Decision points — confirm with the owner before the phase that needs them

- **D1** (Phase 3): legacy tree → `spec/legacy/` for a settling period, deleted later
  (recommended) — or deleted immediately (git history as the only archive).
- **D2** (Phase 0): job 25 converted to tasks (recommended — don't block migration on live
  Naver verification) — or finished in the old system first.
- **D3** (Phase 2): domain map granularity in §5.
- **D4** (Phase 3): be/fe placement gates were *active* triggers ("invoke BEFORE creating
  any file"); ARCH.md is passive reference that implement-task reads. Default: fold and
  delete (owner chose to delete all legacy skills). Accepted tradeoff — placement discipline
  now rides on implement-task reading ARCH.md, so ARCH.md must keep the audit checklists
  prominent.
- **D5** (Phase 3): commit the installed package skills + `.haeram-spec-creator-lock.json`
  (recommended), optionally add `haeram-spec-creator check` to CI to catch drift.

## 8. Risks & mitigations

- **R1 — knowledge compression loss** (18 plans + 17 policies + 11 tech docs → ~13 SSOTs):
  per-domain QA checklist (Phase 2.2), legacy/ retained until narrative read-through passes,
  git history unchanged.
- **R2 — parallel sessions mid-migration**: same-tree sessions are the norm here. One
  dedicated session per phase; freeze banner in Phase 0; domain claims in STATE; explicit
  path staging; small per-phase commits to main.
- **R3 — losing active-trigger placement gates** (see D4).
- **R4 — interview-heavy skills vs bulk conversion**: seed every create-ssot/create-task run
  with the legacy source paths so the interview confirms instead of re-asking; calibration
  (`cfg`) set once in Phase 1 keeps questions at the right depth.
- **R5 — task-id confusion**: legacy T017–T021 (job 25) vs fresh `T###` — new numbering is
  authoritative; legacy ids appear only as references in task bodies.

## 9. Execution rules

- Commits go straight to main (repo convention), one commit per phase minimum, staging
  explicit paths only.
- Never edit the 6 package-managed skill folders by hand; changes ship through the package
  (`pnpm add -D haeram-spec-creator@<next>` + `install`, drift caught by `check`).
- Any step that finds the repo diverging from this plan (e.g. §6 states moved) updates this
  file first, then proceeds.
