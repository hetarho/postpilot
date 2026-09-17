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
| T230 | 형식 안내 and 원문 teach the outline grammar | CLIP | T227 | todo |

## next
- implement-task T230 — the format guide is the last of CLIP r33
- T177 stays blocked (its viewing checklist predates both the caption style set and the outline) and T008 is another session's
## log
- 260918 T229 done; the editor is name, description and one outline the builder and 원문 share — no design step, every entry reorderable and deletable, region lines added one at a time — and ① narrows a bound answer with its own preset (the CLIP-117 gap T227 opened)
- 260918 T229 claimed (otl)
- 260918 T231 done; the pace and the accent stop being seeded too, the FE stops copying the five onto the draft, and an r23 design-first body is pinned through store → converted read → save → mint
- 260918 T231 claimed (otl)
- 260918 T233 done; the outline's caption entries reach the narration call in order and come back placed by the writer, with a fixed entry's own words kept and marked authored so V11 leaves them alone; an omitted entry notices once and the order is never checked
- 260918 T233 claimed (otl)
- 260918 T228 done; one placement rule gives every region entry its slot offset and its drawn line count, the layout and V20 read it together, and the surplus line is a derived notice that follows ①'s preset instead of a stored one; media-smoke stage green
- 260918 T228 claimed (otl)
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
