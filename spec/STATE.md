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
| T243 | every plan check runs on the server before a render of either kind | CLIP | T242 | todo |
| T247 | ②'s dock is the revision composer over 렌더하기 and 확정하기 | CLIP | T246 T242 | todo |
| T248 | 확정하기 opens the finalization dialog | CLIP | T247 | todo |
| T249 | ②'s reference opens on demand | CLIP | T248 | todo |
| T250 | the browser states whether it can render, and refuses with one reason | CLIP | T242 | todo |
| T251 | the browser composites and encodes the video track | CLIP | T250 | todo |
| T252 | the browser builds and encodes the audio track | CLIP | T251 | todo |
| T253 | the browser muxes its output, measures it and stores it before it is shown | CLIP | T252 T243 | todo |
| T254 | ② runs the browser render with its own progress and cancellation | CLIP | T253 T247 | todo |
| T256 | the narration names one caption style per caption | CLIP CDS | - | todo |
| T257 | the approval surface quotes the longest render its styles can produce | CLIP CDS | - | todo |

## next
- implement-task T243 — the next dependency-ready todo in table order; T247 and T250 are also unblocked by T242.
- T256 and T257 remain independent of the browser cluster.
- T177 stays blocked; T008 belongs to another session; post-quality-and-related-links awaits conversion.
## log
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
- 260918 ideation post-quality-and-related-links start
- 260918 T255 claimed (knd)
- 260918 T242 claimed (knd)
- 260918 create-task CLIP done; r37 lands on the todo tasks rather than as new ones — T242 gains the project's last-rendered kind (read off the latest result, no new column), T247 derives the first-offered kind from it with server as the fallback, T250 drops to two refusals and T251 takes the fetch of an unheld original plus CLIP-159's motion over the server's representative raster; T243 T248 T249 T252 T253 T254 are base bumps, T253 and T254 each gaining one note from CLIP-157 and CLIP-126
- 260918 T255+ the release smoke's continuation still expects a generation to produce a result; reproduced at 191e2f9d as `retained candidate failed <nil>` (cmd/api/clip_release_test.go:214, ~64 s inside --target clip-input-smoke), so CLIP-151 is the truth and the smoke is stale
- 260918 create-task CLIP start (r37 delta onto T242 T250 T251, plus the failing release smoke as its own task)
- 260918 implement-task stopped before claiming anything: every todo task is base:CLIP@36 against rev 37, and T242 — the only one whose dep is met — sits inside r37's delta, so create-task CLIP (r37) comes first
- 260918 out of scope, CONFIRMED by local `pnpm smoke:input`: the release smokes still require a generation to leave a result file — `clip_release_test.go:214` (and :802) fail on `Result == nil` with the job done and no failure, which is exactly CLIP-151 as T241 implemented it (ARCH-38 keeps the rollout, which is green); both smokes have to start a render after the plan, so the repair belongs with T242 and needs a task
- 260918 T246 done; the scrubber, one info control and the icon download are the preview's own row, and `Slider` gained a labelless one-row shape for it — the parity copy left `ClipDraftPreview` with the `precise` state its block was the only reader of, and `FinalizeClipNotices` lost its notice list but keeps the refusal and the uncertain retry until T248's dialog takes them
- 260918 T246 claimed (sht)
