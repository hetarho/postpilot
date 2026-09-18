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
| CLIP | 35 | 34 | CLIP-153✎ CLIP-154✎ CLIP-155+ CLIP-156+ — the browser render only, held on the CLIP-76 gap | 4 |
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
| T241 | a generation stops at the validated plan and renders nothing | CLIP | - | todo |
| T242 | a render is started per kind and records the kind with its result | CLIP | T241 | todo |
| T243 | every plan check runs on the server before a render of either kind | CLIP | T242 | todo |
| T244 | ②'s timeline reads at any cut length | CLIP | - | todo |
| T245 | a selected cut or caption opens its own sheet | CLIP | T244 | todo |
| T246 | the preview carries its scrubber, its info control and its download | CLIP | T245 | todo |
| T247 | ②'s dock is the revision composer over 렌더하기 and 확정하기 | CLIP | T246 T242 | todo |
| T248 | 확정하기 opens the finalization dialog | CLIP | T247 | todo |
| T249 | ②'s reference opens on demand | CLIP | T248 | todo |

## next
- update-ssot CLIP: CLIP-154 says a browser render's file is never uploaded while CLIP-76 preserves the confirmed result for preview/download and CLIP-21 keeps it until deletion — a browser-rendered clip has no file for the server to keep, and the browser cluster cannot be planned until that is settled
- implement-task T241, then T242→T243 (the contract) and T244→T249 (②'s surfaces); the two chains are independent until T247, which needs T242's render kind
- T177 stays blocked (its viewing checklist predates the caption style set, the outline and now ②'s shape) and T008 is another session's
## log
- 260918 create-task CLIP done; T235–T240 discarded and r34+r35 re-cut as T241 the plan-only generation, T242 the recorded render kind, T243 the pre-render checks, T244 the timeline, T245 the item sheets, T246 the preview's own controls, T247 the docked composer with 렌더하기, T248 the finalization dialog, T249 the reference sheet — the browser render (CLIP-153 CLIP-154 CLIP-155 CLIP-156) is NOT tasked, see next
- 260918 browser render decided as WebCodecs + an mp4 muxer rather than ffmpeg.wasm (owner, 260918): hardware-accelerated and tens of KB against ffmpeg.wasm's tens of MB and its COOP/COEP requirement, at the cost of a different pipeline from the server's, which pushes CLIP-157 toward output-contract parity rather than pixel parity
- 260918 create-task CLIP start (r35, re-planning T237 T238 T239 with it)
- 260918 T234 done; ②'s draft already autosaved, so 저장 just went and 다시 렌더 flushes the queue itself — which exposed `useGenerateClip.render` refusing the very revision the flush had just won, since the project prop lags the cache write by one render
- 260918 update-ssot CLIP r35 done; a generation now stops at the plan and renders nothing, ② reviews the plan and then the render it asked for, and a render is a browser or a server one chosen per render — both under the same output contract and both able to finalize, the server checking everything the plan can tell before either starts and the producing side measuring its own file, a browser that cannot render refusing rather than changing kind, and the kind recorded on every render while both stay credit-free
- 260918 r35 lands on ②'s dock and its render action, which T237 T238 T239 already rewrite; T234 (doing) touches CLIP-39, whose only change is the action's name
- 260918 update-ssot CLIP start — the render moves behind the owner's approval, ② reviews the plan and then the rendered result, and rendering splits into a browser and a server kind
- 260918 T234 claimed (rfn)
- 260918 create-task CLIP done; r34 becomes T234 the autosaved draft, T235 the readable timeline, T236 the item sheets, T237 the preview's own controls, T238 the finalization dialog, T239 the docked revision composer, T240 the reference sheet — one linear chain because every one of them edits ②'s workspace
- 260918 create-task CLIP start (r34)
- 260918 bugfix: production's first revision request died in prepare with an unnamed reason — the reservation guard, the metered boundary, the ledger's hold/settle/record, the clip accounting read, its SQL and the table's two cancellation CHECKs all named generate_clip alone; they now ask ChargedClipKind, migration 0063 widens the CHECKs (NO TRANSACTION, 0027's pragma, because two tables cascade from generation_jobs), and the reserve/meter/settle, the cancel and the rebuild are pinned by tests
- 260918 update-ssot CLIP r34 done; ② stops being one page holding everything — the editing surface is the preview, its scrubber and the timeline, a selected cut or caption opens a sheet carrying its own frame, the draft autosaves so 저장 is gone, the dock becomes the revision composer with 다시 렌더 and 확정하기 above it, the download moves under the video as an icon, and the parity copy, the project-wide notices and the confirmation copy move into an info control and a finalization dialog
- 260918 no doing task touches ②'s surfaces; T177's viewing checklist is again in the changed area and stays blocked
- 260918 update-ssot CLIP start — ②'s mobile shape: the correction surfaces, the download, the revision request and the dock
- 260918 T230 done; the copied guide teaches the outline and nothing it retired, the example carries stages and a caption, and a paste refuses only unreadable grammar — found and fixed an editor crash on a body with more region lines than the preset holds
- 260918 T230 claimed (otl)
- 260918 T229 done; the editor is name, description and one outline the builder and 원문 share — no design step, every entry reorderable and deletable, region lines added one at a time — and ① narrows a bound answer with its own preset (the CLIP-117 gap T227 opened)
- 260918 T229 claimed (otl)
- 260918 T231 done; the pace and the accent stop being seeded too, the FE stops copying the five onto the draft, and an r23 design-first body is pinned through store → converted read → save → mint
- 260918 T231 claimed (otl)
