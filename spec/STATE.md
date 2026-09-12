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
| CDS | 5 | 5 | - | 2 |
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
- Future overlay designs can use the T119 asset catalog and docs/design/overlay-presets.md; layout/metric changes must also update the adapter and its manifest contract. Remaining visual-contract gaps are listed in docs/audits/recent-commits-260912.md; the current appearance remains unapproved.
- T110 remains owner-blocked: privately load a 9:16 clip with both cards in the Naver picker without publishing, capture the clip-tab overlays and supply the images for CDS-11 measurement. Remaining CDS interpretations are recorded in the results of T107, T108, T109 and T115 (contrast/card geometry, audio seams, exposure and frequency, footage-bound duration).
- T008 needs the owner present: refresh PUB@5 / ARCH@2, rerun postpilot-agent setup before installation for driver signature smarteditor-one-20260910-a6, and cancel or deliberately reuse the queued 20260905-test job; after completion, update PUB for VIDEO-17 and TMPL-39.

## log
- 260912 T119 done (svg); audited 67 commits and corrected CLIP/CDS/PUB/MODEL documentation; file-based SVG catalog preserves default pixels and accepts new trusted presets; all local gates and 27 release scenarios pass, both eight-original videos match the T118 SHA-256 at 512 MiB / 2 CPU, 146.607/135.166 s, no OOM, $0; artifacts retained for review, no new visual approval or deployment
- 260912 create-task and implement-task T119 claimed (svg); file catalog, typed overlay views and identical-output verification under current visual policy
- 260912 update-ssot CLIP CDS PUB MODEL done (svg); document existing duration/style/admission/list-refresh behavior and remove stale unbuilt claims; create-task consumed these documentation-only deltas with no code impact; T008 must read PUB r5 metadata correction on resume, and T110 must read CDS r5 style exception on resume
- 260912 update-ssot CLIP CDS audit start (svg); audit all recent commits for undocumented behavior, then build file-based SVG preset infrastructure while preserving the shipped visual rules
- 260912 T118 done (rpr); owner explicitly authorizes commit/push of the verified rendering, loudness, diagnostics and list-refresh fixes after local playback review; overlay redesign and directory-based SVG presets remain separate and unimplemented
- 260912 owner rejects the original-footage preview's overlay design; source-video quality is acceptable, but test-only location text, disclosure pill and fixed box typography do not meet the intended result; inspect CDS-23/28/29/30/31/51 and retain T118's technical fixes without inferring visual approval
- 260912 T118 blocked on requested owner playback review (rpr); final 20 s original-footage artifacts pass at 512 MiB, 128.801/129.764 s, -16.01 LUFS, no OOM, $0; all local checks and 27 release scenarios pass; changes remain uncommitted and undeployed
- 260912 T118 claimed (rpr); reproduce render failure within shared-server RAM, reduce rendering latency, add safe diagnostics and provide local playback for owner review
- 260912 clip render reliability investigation start (rpr); inspect production failures and elapsed times over SSH, reproduce with the eight originals under production resources, and deliver a local video for owner review before declaring success
- 260912 T117 done (ovl); overlap no longer blocks generated or manual clips, including furniture and final verification; both 30 s renders of the eight originals pass, external model cost $0; production media and release smokes and local CI checks pass
- 260912 T117 claimed (ovl)
- 260912 create-task CDS done (ovl); r4 → T117, overlap advisory throughout delivery
- 260912 create-task CDS start (ovl)
- 260912 update-ssot CDS done (ovl); r4 makes overlap advisory throughout delivery, including furniture and manual plans; other checks remain enforced
- 260912 update-ssot CDS start (ovl); overlap becomes advisory in automatic generation and manual rerendering
- 260912 clip overlap hotfix start (ovl); investigate caption collision failures and verify non-blocking delivery against the eight local source videos
- 260912 T116 done (fbl); a compiled plan whose manifest fails a check walks CDS-55's ladder before any download — style → 깔끔하게, anchor → the style's default, drop — at both pre-download verify points (`Render` and `Layout`, which now returns the repaired plan the composer stores); the verifier names the failing caption's slot (`design.Failure`, `clip.LayoutError`), furniture fails at once, a person's plan is refused untouched, the post-sample contrast fallback is rung 1 of the same ladder, every rung is recorded on `Composition.Fallback`, and the post-render verify stays hard (RENDER.md)
- 260912 T115 done (fbl); CDS-37 is a target: `design.CutBounds(scene, preset)` reads the preset's range, `holdCutLengths` never refuses and `plan_cut_length` is gone, the reconciliation runs a target pass then a footage/fade-floor pass so a reachable timeline is never refused; the recorded single take compiles again. The release gate is GREEN again: the renderer's per-cut source requests are served from one held download (`renderLoader`), the timing fixture reads the 음식점 preset's ends, the workspace bound counts T107's card measurement, and the speech probes map through the persisted plan
