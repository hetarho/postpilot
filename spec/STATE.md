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
| GEN | 8 | 8 | - | 0 |
| MODEL | 16 | 16 | - | 0 |
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
| T313 | Verify complete retirement and preserved manual publishing workflow | PUB EXPORT POST MKT AUTH ARCH | T284 | todo |

## next
- MODEL r16 and GEN r8 are fully built; no model-comparison work remains
- implement-task T313; retirement order T313 → T279; T008 must not resume
- create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta
## log
- 260922 T284 done; companion module, publishing contracts and build hooks removed; retired enum identities reserved and two-runtime generation/build gates pass (ret)
- 260922 T319 done; a lab write comparison keeps its preparing observation in the snapshot, so a comparison on a finalized post changes nothing; every backend package passes, cmd/api at 690s needing a raised limit (obs)
- 260922 T318 done; the A/B pair writes as it is chosen, with no save action; the pair form, model, generation and editor suites and every frontend gate pass (pr)
- 260922 T319 claimed (obs)
- 260922 create-task MODEL GEN complete; T319 created from r16/r8
- 260922 create-task MODEL GEN start; r16/r8 delta
- 260922 update-ssot MODEL GEN complete; MODEL r16 adds MODEL-66 and GEN r8 amends GEN-19: a lab comparison keeps its observation in the snapshot and writes nothing to the post; create-task pending
- 260922 defect found: a lab write comparison persisted its preparing observation onto the source post whatever its status, so running one on a finalized post changed it; MODEL-66 and GEN-19 now forbid it
- 260922 update-ssot MODEL GEN start; a lab comparison must not write to its source post at start
- 260922 T318 claimed (pr)
- 260922 create-task MODEL complete; T318 created from r15
- 260922 create-task MODEL start; r15 delta
- 260922 update-ssot MODEL complete; MODEL r15 adds MODEL-65, the A/B pair saves as it is chosen; create-task pending
- 260922 update-ssot MODEL start; the A/B pair saves as it is chosen
- 260922 T284 claimed (ret)
- 260922 T283 done; publishing backend/runtime removed, migration 0076 enforces a clean checkpoint before dropping five tables, and ARCH-26 plus SQL generation pass; production rollout remains pending (ret)
- 260922 T315 T316 T317 done; windowed me/all leaderboards, the verdict badge sheet and per-row badge tallies. Committed as one change: the three share every backend file they touch. Every package and gate passes on its own; the backend's whole-suite migration failures and the frontend's clip/plans failures reproduce only under load (brd bdg tly)
- 260922 T283 claimed (ret)
- 260922 T312 done; verified evidence-gated object-first cleanup, retry receipt and foreign-key-ordered row purge; focused retirement gates pass and the contended clip regression passes alone (ret)
- 260922 T317 claimed (tly); T315 and T316 are code-complete with gates running, and share every backend file they touch
