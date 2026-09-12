# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 2 | 2 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 9 | 9 | - | 0 |
| POST | 6 | 6 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 6 | 6 | - | 0 |
| MODEL | 10 | 10 | - | 0 |
| TMPL | 6 | 6 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 10 | 10 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 9 | 9 | - | 0 |
| CDS | 9 | 9 | - | 2 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T110 | Release QA on the Naver app and the overlay measurement | CDS CLIP | T108 | blocked@260912 |

## next
- T110 remains owner-blocked: privately load a 9:16 clip with both cards in the Naver picker without publishing, capture the clip-tab overlays and supply the images for CDS-11 measurement. Remaining CDS interpretations are recorded in the results of T107, T108, T109 and T115 (contrast/card geometry, audio seams, exposure and frequency, footage-bound duration).
- T008 needs the owner present: refresh PUB@5 / ARCH@2, rerun postpilot-agent setup before installation for driver signature smarteditor-one-20260910-a6, and cancel or deliberately reuse the queued 20260905-test job; after completion, update PUB for VIDEO-17 and TMPL-39.

## log
- 260912 T123 done (badg); optional disclosure and symmetric header margins verified, both eight-original 20 s renders inspected at 512 MiB / 2 CPUs with no OOM and $0; local checks pass; owner authorized commit/push of T122/T123
- 260912 T123 checks (badg); persisted visibility, quote invalidation, generation/rerender and active-job guard pass; 1,627 FE tests and backend/agent gates pass; original-footage shown video inspected, hidden and production gates running
- 260912 T123 claimed (badg); create-task consumed CLIP/CDS r9; implement visibility, header bounds and verified commit/push
- 260912 create-task CLIP CDS r9 start (badge)
- 260912 update-ssot CLIP CDS r9 done (badge); optional default-on disclosure and symmetric header margins
- 260912 update-ssot CLIP CDS start (badge); owner approved T122 video; add optional disclosure visibility and symmetric header edges, then commit and push approved changes
- 260912 T122 done (pace); final blank-caption correction audit passes; 1,626 FE tests, backend/agent checks and 27 release scenarios pass; local eight-original comparison ready for owner review, $0
- 260912 T122 final fallback audit (pace); ensure a compiler-dropped caption remains editable without placement metadata
- 260912 T122 checks passed (pace); independent typography/pace, exact phrase editing, cropped unique cue layers; all local checks + 27 release scenarios pass; eight-original 20 s comparison, 146.884 s render at 512 MiB / 2 CPU, zero OOM, $0
- 260912 T122 claimed (pace); recipe/caption pace, simple typography, automatic splitting, manual phrase editing and bounded real rendering
- 260912 create-task CLIP CDS r8 done (pace); deltas consumed by T122
- 260912 create-task CLIP CDS r8 start (pace)
- 260912 update-ssot CLIP CDS r8 done (pace); lightweight text, independent rapid pace, phrase editing and bounded rendering; T110 must refresh CDS r8 on resume
- 260912 update-ssot CLIP CDS start (pace); retain stable captions, add a lightweight text treatment and selectable rapid phrase timing with licensed bundled fonts and real-footage review
- 260912 owner accepts the T121 top-inset revision and authorizes commit/push of the current implementation, including the T119/T120 commits; the shorts SVG example remains an optional catalog
- 260912 T121 done (top); vertical inset 40 px plus 40 px gap, regenerated/inspected the 20 s eight-original SVG example at 512 MiB / 2 CPU; 142.355 s render, zero collisions/OOM, $0; local CI checks pass
- 260912 T121 claimed (top); update shared vertical layout and render the revised owner-footage example
- 260912 create-task CDS r7 done (top); delta consumed by T121
- 260912 create-task CDS r7 start (top)
