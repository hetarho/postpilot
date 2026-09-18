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
| CLIP | 34 | 34 | - | 3 |
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
| T234 | ②'s edit draft autosaves and 저장 leaves the dock | CLIP | - | todo |
| T235 | ②'s timeline reads at any cut length | CLIP | T234 | todo |
| T236 | a selected cut or caption opens its own sheet | CLIP | T235 | todo |
| T237 | the preview carries its scrubber, its info control and its download | CLIP | T236 | todo |
| T238 | 확정하기 opens the finalization dialog | CLIP | T237 | todo |
| T239 | ②'s dock is the revision composer over 다시 렌더 and 확정하기 | CLIP | T238 | todo |
| T240 | ②'s reference opens on demand | CLIP | T239 | todo |

## next
- implement-task T234, then T235→T240 in that order — they are one linear chain through ②'s surfaces, so a later one rebases on the earlier one's shape
- T177 stays blocked (its viewing checklist predates the caption style set, the outline and now ②'s shape) and T008 is another session's
## log
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
