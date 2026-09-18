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
| T251 | the browser composites and encodes the video track | CLIP | T250 | todo |
| T252 | the browser builds and encodes the audio track | CLIP | T251 | todo |
| T253 | the browser muxes its output, measures it and stores it before it is shown | CLIP | T252 T243 | todo |
| T254 | ② runs the browser render with its own progress and cancellation | CLIP | T253 T247 | todo |
| T256 | the narration names one caption style per caption | CLIP CDS | - | todo |
| T257 | the approval surface quotes the longest render its styles can produce | CLIP CDS | - | todo |

## next
- implement-task T251 — composite and encode the browser video track.
- T256 and T257 remain independent of the browser cluster.
- T177 stays blocked; T008 belongs to another session; post-quality-and-related-links awaits conversion.
## log
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
- 260918 create-task CLIP CDS start (r38/r23: a style per caption, the sequence quote)
- 260918 ideation post-quality-and-related-links: 추천글 is a template position carrying its own count, tag-matched, excluding links used in the last N posts, rendered as title plus URL and filled by code after validation from the verified Naver URLs PUB-15 already keeps, the model never seeing one; quality is measure-then-offer over the account's recent published posts with four metrics as pass/warn badges and no composite score; research corrected two premises — 도배율 is 블라이's label rather than Naver's, and Naver's own spam page names template-driven bulk publishing and repeated identical links, which reversed the free-repeat and bare-URL decisions
- 260918 update-ssot CDS CLIP done; r38/r23 — the narration names a style per caption so an allowed set wider than one finally renders as a mix, and CLIP-145 closes with no ceiling: the styles are named after approval, so the approval surface quotes the longest render the selection admits rather than a count nobody can know yet
- 260918 update-ssot CDS CLIP start (caption style assignment: an allowed set wider than one still renders as one)
- 260918 T242 released back to todo before any code was written (knd stopped at the owner's word); nothing of it is in the tree
- 260918 T255 done; the smoke's continuation now succeeds on the saved plan with the result untouched, stated by one `continuationVerdict` a host test pins so the next drift breaks ARCH-26 instead of the deploy — clip-input-smoke is green again (PASS in 70 s)
