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
| T254 | ② runs the browser render with its own progress and cancellation | CLIP | T253 T247 | todo |
| T256 | the narration names one caption style per caption | CLIP CDS | - | todo |
| T257 | the approval surface quotes the longest render its styles can produce | CLIP CDS | - | todo |

## next
- implement-task T254 — connect browser rendering to the correction dock, progress and cancellation.
- T256 and T257 remain independent of the browser cluster.
- T177 stays blocked; T008 belongs to another session; post-quality-and-related-links awaits conversion.
## log
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
- 260918 T242 done; render kind survives the result and completion staging, old results read as server, and project reads expose the last successful kind.
- 260918 out of scope: frontend/src/pages/editor/ui/DraftEditor.tsx:140 has an existing storedAnswers useMemo dependency warning; lint passes with no errors.
- 260918 T242 claimed (rnd)
- 260918 create-task CLIP CDS done; T256 reverses the narration contract so a caption carries its own style (owner > narration > default, out-of-set falls back with one notice and spends no correction), T257 turns the pre-generation quote from a style count into the longest render the selection admits — neither depends on the other and both are independent of the browser cluster
