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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 2 | 2 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 12 | 12 | - | 0 |
| POST | 6 | 6 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 6 | 6 | - | 0 |
| MODEL | 10 | 10 | - | 0 |
| TMPL | 6 | 6 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 12 | 12 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 18 | 18 | - | 0 |
| CDS | 13 | 13 | - | 2 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T110 | Release QA on the Naver app and the overlay measurement | CDS CLIP | T108 | blocked@260912 |
| T146 | Compose validated split and fixed-rate clip assemblies | CLIP CDS ARCH | T142 T143 T145 | todo |
| T147 | Render fixed-rate cuts and opted-in source audio | CLIP CDS ARCH | T142 T143 | todo |
| T148 | Preview the transformed assembly timeline | CLIP CDS ARCH | T142 T143 | todo |
| T149 | Add cut, split and fixed-rate editing controls | CLIP CDS ARCH | T144 T148 | todo |
| T150 | Add per-source original-sound toggles | CLIP CDS ARCH | T143 T148 | todo |
| T151 | Verify review-clip assembly end to end | CLIP CDS ARCH | T146 T147 T149 T150 | todo |
| T153 | Admit the empty legacy disclosure a composition project saves with | ARCH | - | doing@260914.disc |

## next
- T152 is done; deploy it before the next owner generation attempt, since production still refuses correct renders until it ships
- T145 is done, so T146 is unblocked; T147 T148 T149 T150 were already unblocked
- T110 remains owner-blocked and must refresh CLIP@18 / CDS@13 before resuming; T008 remains owner-dependent for its separate local Naver publication verification.

## log
- 260914 T145 done (asm); clip-observation-v2 records every chunk completely with action/motion and an explicit certainty/usability, refuses instead of clamping model times, and never promotes v1 evidence into a v2 generation
- 260914 T153 claimed (disc)
- 260914 create-task review/clip-project-update-260914 done; T153 fixes composition-project update admission
- 260914 create-task review/clip-project-update-260914 start
- 260914 T145 claimed (asm)
- 260914 T144 done (rate); request-only creation provenance admits owner add and split through the existing optimistic save, with observed-scene evidence, server-owned defaults and named refusals
- 260914 T144 claimed (rate)
- 260914 T143 done (rate); migration 0052 backfills legacy audio meaning, one writer transaction owns the lease/plan/revision/retention change, and the setting stays outside every paid identity
- 260914 T143 claimed (rate)
- 260914 T142 done (rate); v6 assembly envelope with per-cut fixed rates and the complete owner source-audio snapshot, one checked transformed-duration helper, cadence-verified slow rates and legacy-overlap grandfathering
- 260914 T152 done; delivered length read from the decoded video track, production job b12a4bcd output verified accepted
- 260914 T142 refreshed to CDS@13 (rate); the r13 V12 decoded-length delta is T152 render conformance and does not touch the assembly contract; T143 T144 rebased too
- 260914 T152 claimed (vdur)
- 260914 create-task CDS done; T152 moves the delivered-length measure onto the decoded video track
- 260914 create-task CDS start
- 260914 update-ssot CDS r13 done; delivered clip length is read from the decoded video track, not the padded container or audio declaration
- 260914 warning; T142 doing is based on CDS@12 and T147 todo carries the V12 render check, both must refresh to CDS@13
- 260914 update-ssot CDS start
- 260914 T142 claimed (rate)
- 260914 create-task CLIP CDS done; T142-T151 cover versioned rate/audio contracts, complete observation, assembly writing, deterministic render, owner editing and release QA
