# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ideation
| id | st |
|---|---|
| clip-source-observation-visibility | converted@260912 |
| clip-template-as-preset | converted@260917 |
| post-quality-and-related-links | open@260918 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 7 | 7 | - | 0 |
| AUTH | 7 | 7 | - | 0 |
| QUOTA | 19 | 19 | - | 0 |
| POST | 9 | 9 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 7 | 7 | - | 0 |
| MODEL | 14 | 14 | - | 0 |
| TMPL | 8 | 8 | - | 1 |
| GUIDE | 3 | 3 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 17 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 43 | 40 | CLIP-13✎ | 1 |
| CDS | 25 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 1 | 1 | - | 2 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |
| publishing-260922 | converted@260922 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | Superseded by PUB r6; original live-smoke claim retained, do not resume | PUB MKT | T007 T046 | doing@260910.e2e |
| T177 | Release QA viewing checklist | CDS CLIP | T176 | blocked@260916 |
| T279 | Pin enum mirrors in the surviving backend and frontend | ARCH | T313 | todo |
| T282 | Stop and uninstall existing Mac publishing companions | PUB ARCH | T311 | todo |
| T283 | Remove publishing backend, deletion guards and database tables | PUB POST AUTH QUOTA LANG ARCH | T312 | todo |
| T284 | Delete companion source, publishing contracts and build integration | PUB AUTH LANG VIDEO ARCH | T283 | todo |
| T311 | Remove publishing screens, polling and product promises | PUB EXPORT POST MKT QUOTA LANG VIDEO ARCH | T310 | doing@260922.ret |
| T312 | Export retirement outcomes and purge publishing-owned data | PUB ARCH | T282 | todo |
| T313 | Verify complete retirement and preserved manual publishing workflow | PUB EXPORT POST MKT AUTH ARCH | T284 | todo |
| T315 | Leaderboards keyed by scope, stage and window | MODEL LANG ARCH | - | todo |
| T316 | A winner verdict carries badges through one confirmation sheet | MODEL ARCH THEME | T314 | todo |
| T317 | Leaderboard rows tally their badges | MODEL ARCH | T315 T316 | todo |

## next
- implement-task T315 (independent) or T316 (T314 is done); then T317 last
- implement-task T311; retirement order T311 → T282 → T312 → T283 → T284 → T313; T008 must not resume, T279 follows removal
- create-task MKT THEME for the existing /about header overflow at 320px/200% text
- create-task CLIP CDS THEME for the r41/r24 Wanted Sans delta, which T305..T309 did not consume
## log
- 260922 T314 done; a comparison freezes its origin, a lab verdict picks without applying and its follow-ups are gated to draft/review, migration 0073 backfills; whole backend suite, frontend gates and build pass, clip suite failures are the known whole-suite flake (org)
- 260922 T314 claimed (org)
- 260922 T311 claimed (ret)
- 260922 T310 done; migration 0072 permanently refuses automatic publishing, revokes capabilities, conservatively settles jobs and exports a private retirement report; backend and spec gates pass (ret)
- 260922 create-task MODEL LANG complete: T314 origin-aware verdicts, T315 windowed me/all leaderboards (consumes LANG-18), T316 badge sheet, T317 leaderboard tallies; MODEL tasked 14, LANG tasked 5
- 260922 create-task MODEL LANG start; MODEL r14 and LANG r5 deltas
- 260922 LANG r5 changes only LANG-18's leaderboard key; T283 T284 T311 (base LANG@4) are unaffected in scope and need no re-planning
- 260922 update-ssot MODEL LANG complete; MODEL r14 (lab picks apply nothing, follow-ups gated to draft/review, `(scope, stage, window)` leaderboards, verdict badges), LANG r5 syncs the leaderboard key; create-task pending
- 260922 update-ssot MODEL start; model-lab verdicts pick only, windowed me/all leaderboards, verdict badges
- 260922 T310 claimed (ret)
- 260922 retirement planning validation passed: 10 SSOT revisions/change entries, 8 task references/bases, acyclic dependencies and STATE consistency; T008 and unrelated pending preserved; review adoption synchronized; implementation not started
- 260922 create-task retirement complete: T310/T311/T312/T313 created, T282/T283/T284 repurposed for removal, T279 narrowed to surviving enums; all 10 retirement SSOT deltas consumed, unrelated pending retained
- 260922 TMPL r8 consumed without a standalone implementation task: only removes the retired-agent dependency from the still-open photo-row question; PUB-49 is covered by T313
- 260922 create-task PUB ARCH AUTH QUOTA POST EXPORT MKT VIDEO TMPL LANG start; plan phased removal and replace obsolete todo agent refactors
- 260922 update-ssot PUB ARCH AUTH QUOTA POST EXPORT MKT VIDEO TMPL LANG complete; phased retirement adopted, content/manual export preserved; create-task pending
- 260922 warning: T008 doing claim is superseded by PUB r6; do not resume live publishing or use T008 as a retirement dependency; its task file remains immutable
- 260922 create-architecture ARCH retirement alignment start; remove the companion architecture through the PUB cutover stages
- 260922 update-ssot PUB and dependent domains start; approved phased retirement of automatic publishing, followed by create-task planning
- 260922 review-code publishing-260922 complete; recommend retiring automatic Naver publish and retaining export, based on official automation restrictions and missing live acceptance; no adoption or implementation
- 260922 review-code publishing-260922 start; assess publishing viability and Naver automation/account restrictions
