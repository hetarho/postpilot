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
| T147 | Render fixed-rate cuts and opted-in source audio | CLIP CDS ARCH | T142 T143 | todo |
| T148 | Preview the transformed assembly timeline | CLIP CDS ARCH | T142 T143 | todo |
| T149 | Add cut, split and fixed-rate editing controls | CLIP CDS ARCH | T144 T148 | todo |
| T150 | Add per-source original-sound toggles | CLIP CDS ARCH | T143 T148 | todo |
| T151 | Verify review-clip assembly end to end | CLIP CDS ARCH | T146 T147 T149 T150 | todo |

## next
- T153 is deployed-ready; composition-project editing was blocked in production until it ships
- T146 is done but MUST NOT ship alone: the renderer still refuses a non-1x plan until T147 lifts that gate
- T110 remains owner-blocked and must refresh CLIP@18 / CDS@13 before resuming; T008 remains owner-dependent for its separate local Naver publication verification.

## log
- 260914 T146 done (asm); one rate per cut from the source's own allowed set, same-scene-only splits with no reuse, the whole timeline on transformed output time and the owner sound snapshot stamped after validation
- 260914 update-ssot CLIP start
- 260914 T153 done; composition projects can be saved again, legacy campaign identity still required
- 260914 flaky under full-suite load, not in T153 scope; FE ClipCorrection 'saves exact milliseconds' and BE store recovery-restart fail intermittently while passing in isolation
- 260914 T146 refreshed to CDS@13 (asm); the r13 CDS-52 delta is delivered-length render conformance and touches no assembly-writer decision
- 260914 T146 claimed (asm)
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
