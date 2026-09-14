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
| T157 | Say a save was refused rather than promising a retry that was abandoned | ARCH | - | todo |

## next
- T156 is complete and verified; T157 completes the failure-visibility release batch.
- T110 stays owner-blocked and T008 owner-dependent.

## log
- 260915 T156 done (vis); all 97 public checks explained beside failures in ko/en
- 260915 T156 claimed (vis)
- 260915 T155 done (grp); named controls, minimum display without opening writes, maximum and editor round trips verified
- 260915 T155 claimed (grp)
- 260915 T154 done (grp); compatible group declarations and pre-work admission verified
- 260914 T154 claimed (grp)
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
