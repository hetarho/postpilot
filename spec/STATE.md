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
| CLIP | 7 | 7 | - | 0 |
| CDS | 7 | 7 | - | 2 |
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
- 260912 owner accepts the T121 top-inset revision and authorizes commit/push of the current implementation, including the T119/T120 commits; the shorts SVG example remains an optional catalog
- 260912 T121 done (top); vertical inset 40 px plus 40 px gap, regenerated/inspected the 20 s eight-original SVG example at 512 MiB / 2 CPU; 142.355 s render, zero collisions/OOM, $0; local CI checks pass
- 260912 T121 claimed (top); update shared vertical layout and render the revised owner-footage example
- 260912 create-task CDS r7 done (top); delta consumed by T121
- 260912 create-task CDS r7 start (top)
- 260912 update-ssot CDS r7 done (top); vertical design inset 40 px plus 40 px header gap; platform estimates remain references; T110 must refresh CDS r7 on resume
- 260912 update-ssot CDS start (top); reduce the vertical top inset to 40 px plus the existing 40 px header gap and regenerate the SVG review video
- 260912 T120 done (sht); aligned header glyphs and the shared vertical grid, added an opt-in shorts SVG catalog and retained the final 20 s video plus editable/filled SVGs; all eight originals pass at 512 MiB / 2 CPU, 122.571 s render, zero collisions/OOM and $0; local gates and 27 release scenarios pass, no default-theme rollout
- 260912 create-task and implement-task T120 claimed (sht); measured alignment, common grid and opt-in shorts SVG catalog with real owner-footage review
- 260912 update-ssot CDS r6 done (sht); shared vertical grid and measured header alignment; T110 must refresh CDS r6 on resume; the SVG example stays an opt-in review asset
- 260912 update-ssot CDS start (sht); align disclosure and facts on one header row and one shared vertical grid; create an opt-in SVG shorts example and render owner originals for visual review
- 260912 T119 done (svg); audited 67 commits and corrected CLIP/CDS/PUB/MODEL documentation; file-based SVG catalog preserves default pixels and accepts new trusted presets; all local gates and 27 release scenarios pass, both eight-original videos match the T118 SHA-256 at 512 MiB / 2 CPU, 146.607/135.166 s, no OOM, $0; artifacts retained for review, no new visual approval or deployment
- 260912 create-task and implement-task T119 claimed (svg); file catalog, typed overlay views and identical-output verification under current visual policy
- 260912 update-ssot CLIP CDS PUB MODEL done (svg); document existing duration/style/admission/list-refresh behavior and remove stale unbuilt claims; create-task consumed these documentation-only deltas with no code impact; T008 must read PUB r5 metadata correction on resume, and T110 must read CDS r5 style exception on resume
- 260912 update-ssot CLIP CDS audit start (svg); audit all recent commits for undocumented behavior, then build file-based SVG preset infrastructure while preserving the shipped visual rules
- 260912 T118 done (rpr); owner explicitly authorizes commit/push of the verified rendering, loudness, diagnostics and list-refresh fixes after local playback review; overlay redesign and directory-based SVG presets remain separate and unimplemented
- 260912 owner rejects the original-footage preview's overlay design; source-video quality is acceptable, but test-only location text, disclosure pill and fixed box typography do not meet the intended result; inspect CDS-23/28/29/30/31/51 and retain T118's technical fixes without inferring visual approval
- 260912 T118 blocked on requested owner playback review (rpr); final 20 s original-footage artifacts pass at 512 MiB, 128.801/129.764 s, -16.01 LUFS, no OOM, $0; all local checks and 27 release scenarios pass; changes remain uncommitted and undeployed
