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
| CLIP | 37 | 36 | CLIP-159+ CLIP-157✎ CLIP-155✎ CLIP-153✎ CLIP-126✎ | 2 |
| CDS | 22 | 22 | - | 1 |
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
| T242 | a render is started per kind and records the kind with its result | CLIP | T241 | todo |
| T243 | every plan check runs on the server before a render of either kind | CLIP | T242 | todo |
| T245 | a selected cut or caption opens its own sheet | CLIP | T244 | todo |
| T246 | the preview carries its scrubber, its info control and its download | CLIP | T245 | todo |
| T247 | ②'s dock is the revision composer over 렌더하기 and 확정하기 | CLIP | T246 T242 | todo |
| T248 | 확정하기 opens the finalization dialog | CLIP | T247 | todo |
| T249 | ②'s reference opens on demand | CLIP | T248 | todo |
| T250 | the browser states whether it can render, and refuses with one reason | CLIP | T242 | todo |
| T251 | the browser composites and encodes the video track | CLIP | T250 | todo |
| T252 | the browser builds and encodes the audio track | CLIP | T251 | todo |
| T253 | the browser muxes its output, measures it and stores it before it is shown | CLIP | T252 T243 | todo |
| T254 | ② runs the browser render with its own progress and cancellation | CLIP | T253 T247 | todo |

## next
- create-task CLIP (r37) before T242 or any of T250→T254 is claimed — CLIP-153 now offers the project's last kind first, which T242 never carried, and r37 cuts T250's refusals from three to two while handing T251 the per-style motion; T243 waits behind T242
- implement-task T245 next (②'s surfaces, T244 done); T246→T249 follow it in order
- T177 stays blocked (its viewing checklist predates the caption style set, the outline and now ②'s shape) and T008 is another session's
## log
- 260918 T244 done; every cut draws its thumbnail at once, each ruler tick and label is bounded by its own cut and dropped below CLIP_TIMELINE.minLabelPx, undo/redo head the timeline as icon controls, and the save state is the page's status region alone — hiding a label also hid the control's name, so each bar now carries its own aria-label
- 260918 T241 done; the generation ends on the validated plan, `SaveGeneratedPlan` advances the analysis and the plan while `result_*`/`rendered_plan_revision` stand, and a file-less completion skips the staging row and applies its plan in the job's own terminal transaction
- 260918 out of scope: `renderLoader`'s `verifyRetained` probe lost its only caller with the generation's render stage; the render job does the same identity check itself (review-code candidate)
- 260918 update-ssot CLIP r37 done; the browser render draws every style including the sequence ones — it sets no type and applies only the motion, so the two kinds owe the same clip and not the same file and a caption that moves differently between them is not a defect; the originals a page lacks are fetched rather than refusing the kind, leaving the encoders and the memory as the two refusals; the kind offered first is the project's last one and the browser kind where it has none; CLIP-126 closes with no wall-clock committed
- 260918 r37 rewrites T250's three refusals to two and gives T251 the per-style motion it never carried; T241 and T244 (both doing) are outside it
- 260918 T244 claimed (ctr)
- 260918 update-ssot CLIP start — the browser kind draws only static captions, fetches the originals it lacks, and the kinds owe the same clip rather than the same file
- 260918 T241 claimed (ctr)
- 260918 create-task CLIP done; the browser render becomes T250 the capability refusal, T251 the video track, T252 the audio track, T253 the mux/measure/store, T254 ②'s own progress and cancellation — the server already typesets every caption PNG the draft preview draws, so the browser composites them and never sets type itself
- 260918 create-task CLIP start (the browser render cluster)
- 260918 update-ssot CLIP r36 done; a browser render stores its file before the owner sees it and is unsuccessful until it does, so ② always plays what it will hand over and CLIP-76's preserved result holds for both kinds — the server records the browser's own verdict instead of decoding the stored file again
- 260918 update-ssot CLIP start — a browser render's file reaches the server before the owner sees it
- 260918 create-task CLIP done; T235–T240 discarded and r34+r35 re-cut as T241 the plan-only generation, T242 the recorded render kind, T243 the pre-render checks, T244 the timeline, T245 the item sheets, T246 the preview's own controls, T247 the docked composer with 렌더하기, T248 the finalization dialog, T249 the reference sheet — the browser render (CLIP-153 CLIP-154 CLIP-155 CLIP-156) is NOT tasked, see next
- 260918 browser render decided as WebCodecs + an mp4 muxer rather than ffmpeg.wasm (owner, 260918): hardware-accelerated and tens of KB against ffmpeg.wasm's tens of MB and its COOP/COEP requirement, at the cost of a different pipeline from the server's, which pushes CLIP-157 toward output-contract parity rather than pixel parity
- 260918 create-task CLIP start (r35, re-planning T237 T238 T239 with it)
- 260918 T234 done; ②'s draft already autosaved, so 저장 just went and 다시 렌더 flushes the queue itself — which exposed `useGenerateClip.render` refusing the very revision the flush had just won, since the project prop lags the cache write by one render
- 260918 update-ssot CLIP r35 done; a generation now stops at the plan and renders nothing, ② reviews the plan and then the render it asked for, and a render is a browser or a server one chosen per render — both under the same output contract and both able to finalize, the server checking everything the plan can tell before either starts and the producing side measuring its own file, a browser that cannot render refusing rather than changing kind, and the kind recorded on every render while both stay credit-free
- 260918 r35 lands on ②'s dock and its render action, which T237 T238 T239 already rewrite; T234 (doing) touches CLIP-39, whose only change is the action's name
- 260918 update-ssot CLIP start — the render moves behind the owner's approval, ② reviews the plan and then the rendered result, and rendering splits into a browser and a server kind
- 260918 T234 claimed (rfn)
