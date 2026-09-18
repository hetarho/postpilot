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
| CLIP | 38 | 38 | - | 1 |
| CDS | 23 | 23 | - | 1 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T177 | Release QA viewing checklist | CDS CLIP | T176 | blocked@260916 |
| T257 | the approval surface quotes the longest render its styles can produce | CLIP CDS | - | todo |

## next
- implement-task T257 — quote the longest render the selected caption styles admit.
- T177 stays blocked; T008 belongs to another session; post-quality-and-related-links awaits conversion.
## log
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
- 260918 T243 done; both kinds run server plan admission first, browser verdicts are owner/revision-bound records with notices, and file promotion remains T253.
- 260918 T243 claimed (rnd)
