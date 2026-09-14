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
| clip-release-smoke-260914 | ready@260914 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T110 | Release QA on the Naver app and the overlay measurement | CDS CLIP | T108 | blocked@260912 |
| T154 | Declare an item group's name and admitted count, and refuse a short one before paid work | CLIP | - | todo |
| T155 | Call an item group by its name and open it at its minimum | CLIP | T154 | todo |
| T156 | Show the recorded check beside the failure reason | ARCH | - | todo |
| T157 | Say a save was refused rather than promising a retry that was abandoned | ARCH | - | todo |

## next
- This release batch is complete: T150/T151 are implemented, verified and individually committed.
- T154/T155 form the next item-group admission batch; T156/T157 remain separate failure-visibility work.
- T110 stays owner-blocked and T008 owner-dependent.

## log
- 260914 T151 done (snd); six assembly exports/156 browser frames, recovery/migrations, original cadence and mandatory production-image gates verified
- 260914 T151 refreshed to CLIP@19 CDS@13 (snd); compatible item-group defaults and the existing decoded-duration gate preserve this assembly matrix scope
- 260914 T151 claimed (snd)
- 260914 T150 done (snd); revision-safe source-sound switches, rollback, retry and inverse undo verified
- 260914 T150 refreshed to CLIP@19 CDS@13 (snd); item-group admission and decoded export duration do not alter source-sound controls
- 260914 T150 claimed (snd)
- 260914 T149 done (prv); observed-cut creation, split, fixed-rate editing and revision-safe undo verified
- 260914 T149 refreshed to CLIP@19 CDS@13 (prv); item-group admission and decoded export duration do not alter owner cut editing
- 260914 T149 claimed (prv)
- 260914 T148 done (prv); transformed browser timeline and native rate/pitch/source-audio gating verified
- 260914 T148 refreshed to CLIP@19 CDS@13 (prv); item-group admission and decoded export duration do not change the draft-preview decisions
- 260914 T148 claimed (prv)
- 260914 T158 done (smk); the release smoke's synthetic answers now state the v2 observation fields and CLIP-98's per-cut rate, so the production-image gate stops refusing at the first chunk — the image gate itself did not run here (no Docker daemon), CI's build is the proof
- 260914 T158 claimed (smk)
- 260914 create-task review/clip-release-smoke-260914 done; T158 repairs the production-image gate, F2 left open for ARCH
- 260914 create-task review/clip-release-smoke-260914 start
- 260914 T147 done (asm); one timestamp-scaling rate chain with pitch-preserved atempo, the whole render clock on transformed time, audio only from owner-enabled sources and a cadence recheck that refuses before FFmpeg
- 260914 T147 refreshed to CLIP@19 (asm); r19 changes item-group declaration and pre-work admission, none of which this task's rendering decisions
- 260914 flaky under full-suite load, reproduced on c02fa07 before T145; internal/clip/store generation/recovery tests intermittently fail with "clip sources are not available in this state" while passing in isolation
- 260914 create-task CLIP review/clip-failure-visibility-260914 done; T154 T155 carry the declared group, T156 T157 carry what a refusal tells the owner
