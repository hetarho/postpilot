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
| ARCH | 3 | 3 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 12 | 12 | - | 0 |
| POST | 7 | 7 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 6 | 6 | - | 0 |
| MODEL | 10 | 10 | - | 0 |
| TMPL | 6 | 6 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 13 | 13 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 40 | 40 | - | 1 |
| CDS | 23 | 23 | - | 1 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | ready@260919 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T177 | Release QA viewing checklist | CDS CLIP | T176 | blocked@260916 |

## next
- create-task review/arch-260919 (31 [o]; ARCH amendments for the four noted gaps come first, agent findings F24-F30 wait for T008).
- No claimable todo tasks remain; T008 belongs to another session and T177 remains blocked.
- post-quality-and-related-links remains open ideation, awaiting conversion when ready.
## log
- 260919 review-code arch-260919 ready; owner adopted all 31 (rule: clear anything that accrues per change now); next create-task review/arch-260919
- 260919 review-code arch-260919 FE re-verified at 19c19cc2; F9 widened (6 pages own RPC), F31 added (cross-domain cache keys in 8 features); 31 [?] awaiting triage
- 260919 review-code arch-260919 findings written (30, 1×P1 F13 cmd/api sagas; FE 12 · BE 10 · agent/proto 8); awaiting triage
- 260919 review-code arch-260919 start
- 260918 T257 done; approval quotes the whole target before narration and actual styled captions afterward, in ko/en seconds with no sequence ceiling; all local gates pass.
- 260918 T257 claimed (rnd)
- 260918 T256 done; narration names each caption style, defaults without extra calls, preserves owner/legacy choices and reports out-of-set fallbacks; all local and image gates pass.
- 260918 T256 claimed (rnd)
- 260918 T254 done; browser rendering stays in ② with encode/store progress, cancellation and navigation cleanup; promotion and orphan cleanup serialize without an encoding time limit, and real Chromium plus local/image gates pass.
- 260918 T254 claimed (rnd)
- 260918 T253 done; MP4 timing excludes measured AAC priming, direct immutable uploads promote atomically after stored-file/verdict checks, and failed attempts preserve the prior result; all local and image gates pass.
- 260918 T253 claimed (rnd)
- 260918 T252 done; retained-source audio uses pitch-preserving rates, exact cut timing, BS.1770 normalization and measured AAC priming; Chromium audio and video checks pass.
- 260918 T252 claimed (rnd)
- 260918 T251 done; worker composition encodes exact plan frames with server PNGs, one source read, bounded queues and cancellation cleanup; all three canvases and real-font media smokes pass.
- 260918 T251 claimed (rnd)
- 260918 T250 done; static H.264/AAC probes and reported memory yield one browser refusal, with no original-file check, encoding, network call or implicit kind switch.
- 260918 T250 claimed (rnd)
- 260918 T249 done; one reference sheet lazily mounts observations, sources or requests, and observed cut additions land in the new cut’s editor.
- 260918 T249 claimed (rnd)
- 260918 T248 done; finalization flushes before a targeted-notice confirmation dialog, with render-first and other refusals beside the dock button.
- 260918 T248 claimed (rnd)
- 260918 T247 done; the two-row dock keeps the revision composer reachable, send opens credit approval, and render labels derive the kind and current-plan match.
- 260918 T247 claimed (rnd)
