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
| CLIP | 19 | 19 | - | 1 |
| CDS | 13 | 13 | - | 2 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T110 | Release QA on the Naver app and the overlay measurement | CDS CLIP | T108 | blocked@260912 |
| T148 | Preview the transformed assembly timeline | CLIP CDS ARCH | T142 T143 | todo |
| T149 | Add cut, split and fixed-rate editing controls | CLIP CDS ARCH | T144 T148 | todo |
| T150 | Add per-source original-sound toggles | CLIP CDS ARCH | T143 T148 | todo |
| T151 | Verify review-clip assembly end to end | CLIP CDS ARCH | T146 T147 T149 T150 | todo |
| T154 | Declare an item group's name and admitted count, and refuse a short one before paid work | CLIP | - | todo |
| T155 | Call an item group by its name and open it at its minimum | CLIP | T154 | todo |
| T156 | Show the recorded check beside the failure reason | ARCH | - | todo |
| T157 | Say a save was refused rather than promising a retry that was abandoned | ARCH | - | todo |

## next
- implement-task T154, then T155; T156 and T157 are independent and can go first if the screen matters more than the refusal
- T146+T147 are done and deployable together: the writer states rates and the renderer performs them; T148 is the next unblocked assembly task
- T110 remains owner-blocked and must refresh CLIP@19 / CDS@13 before resuming; T008 remains owner-dependent for its separate local Naver publication verification.

## log
- 260914 T147 done (asm); one timestamp-scaling rate chain with pitch-preserved atempo, the whole render clock on transformed time, audio only from owner-enabled sources and a cadence recheck that refuses before FFmpeg
- 260914 T147 refreshed to CLIP@19 (asm); r19 changes item-group declaration and pre-work admission, none of which this task's rendering decisions
- 260914 flaky under full-suite load, reproduced on c02fa07 before T145; internal/clip/store generation/recovery tests intermittently fail with "clip sources are not available in this state" while passing in isolation
- 260914 create-task CLIP review/clip-failure-visibility-260914 done; T154 T155 carry the declared group, T156 T157 carry what a refusal tells the owner
- 260914 create-task CLIP review/clip-failure-visibility-260914 start
- 260914 update-ssot CLIP r19 done; an item group declares its name and admitted count, and admission refuses a structurally unsatisfiable input set before paid work
- 260914 warning; T147 doing is based on CLIP@18 and must refresh to CLIP@19, though r19 touches admission and item groups rather than rendering
- 260914 T147 refreshed to CDS@13 (asm); the r13 CDS-52 delta is V12's decoded video-track duration, already shipped by T152, and this task extends that same check rather than contradicting it
- 260914 T147 claimed (asm)
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
