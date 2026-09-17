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
| CLIP | 33 | 33 | - | 3 |
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
| T228 | The project's presets decide how many intro and outro lines are drawn | CLIP CDS | T227 | todo |
| T229 | The template editor drops the design step and edits the outline | CLIP | T227 | todo |
| T230 | 형식 안내 and 원문 teach the outline grammar | CLIP | T227 | todo |
| T231 | A saved template reads as an outline and seeds no design | CLIP | T227 | todo |
| T233 | The template's caption entries reach the narration in outline order | CLIP | T228 | todo |

## next
- implement-task T228 (then T229 T230 T231 in any order, T233 behind T228)
- T177 stays blocked (its viewing checklist predates both the caption style set and the outline) and T008 is another session's
## log
- 260918 T227 done; the template body is an ordered outline in both parsers — no design attributes, no interval, no preset conformance, captions admitted — with the shared corpus migrated; T228's preset readers and T231's end of design seeding came with it because the compiler required them
- 260918 T227 claimed (otl)
- 260918 T232 done; 템플릿 없음 is plain metadata in the list and ① already distinguished the deleted-template case, now pinned by tests on both surfaces
- 260918 T232 claimed (otl)
- 260918 create-task CLIP done; T227 the outline grammar in both parsers, T228 the project's presets fill the region lines, T229 the editor, T230 the format guide, T231 the legacy read and the end of design seeding, T232 템플릿 없음, T233 the template's caption entries — CDS r22 is consumed by T228
- 260918 create-task CLIP start (CDS r22 rides along)
- 260918 update-ssot CLIP r33 + CDS r22 done; the template is an ordered outline whose entries declare only their order, the design selection is the project's alone (so the template screen offers no caption style at all), a paste is refused only for unreadable grammar, the project's preset decides how many intro/outro lines are drawn with the surplus noticed, and 템플릿 없음 stops reading as a missing template
- 260918 T177 is in the changed area and stays blocked; no doing task touches the template surfaces
- 260918 update-ssot CLIP start — the template as a post-template-style ordered outline round-tripped through 원문, and a template-less project shown as a normal state
- 260917 T226 done; both approval quotes carry what the frame-by-frame captions add — the plan's sequence captions, their frames and the seconds at a MEASURED per-frame cost (30 ms, resvg over real frames, 6–37 ms by crop on this Mac), or the selected sequence styles before a plan exists — shown on the one approval surface ① and ② share, refusing nothing (CLIP-145 stays open and needs prod numbers)
- 260917 T226 claimed (stg)
- 260917 T224 done; ① chooses the design — intro and outro presets beside pace and accent, the sixteen caption styles as a multi-select where each is drawn by the renderer itself and the frame-by-frame ones say so, an empty selection reading as the default style alone — the template is optional everywhere the FE still demanded one (없음 by default, minting without it), and choosing one fills all five while clearing it keeps them; the style samples needed a new GetClipCaptionStyleSamples (T222's preview needs a saved plan, so ① could not use it) — owner-approved mid-task
- 260917 T224 claimed (stg)
- 260917 T223 done; a template may carry named composition stages (`<stage name="…">한 줄 의도</stage>`, root only, at most 8, both halves required and bounded by the label and prompt counts), read by the Go and TypeScript parsers against the shared corpus and handed to the flow call in the template guide's position as a numbered order to follow where the footage allows — admitting and forbidding nothing, with no notice, count or refusal anywhere for a stage
- 260917 T225 done; ② places each caption over a still frame of the cut it starts in — drag stopping at the safe area it draws while moving, arrow-key nudging in canvas pixels, the size refused outside CDS-3's floor and the role's own size at the control, the style taken from the project's allowed set, a contrast shortfall shown in place and blocking nothing, and a plain ground with the reason where the footage is not here — every change riding the existing draft queue and undo
- 260917 T223 claimed (stg)
- 260917 T225 claimed (plc)
- 260917 T222 done; GetClipCaptionPreview hands ② each caption as the renderer's own drawing with its root taken off — one `<g>` brought to the origin by a transform the reported box cancels exactly, ids prefixed per caption, a sequence style labelled as one representative frame — and a smoke contract rasterises the placed fragment and the renderer's caption to byte-identical PNGs; Jua and NanumMyeongjo now reach the browser too
- 260917 T222 claimed (plc)
- 260917 T221 done; a caption carries the owner's own position, size and style — clamped into the safe area by moving, never resizing, with the size floor and an unallowed style refused where they are written — and the manifest says who placed it so V1 still holds it inside the safe area, V3 demotes a shortfall under it to a notice, V13 leaves it out of the anchor walk and no repair or automatic placement runs over it again
