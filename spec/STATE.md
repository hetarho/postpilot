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
| MODEL | 9 | 9 | - | 0 |
| TMPL | 6 | 6 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 4 | 4 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 10 | 10 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 6 | 6 | - | 0 |
| CDS | 4 | 4 | - | 2 |
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
- T117 shipped CDS r4: overlap is advisory at every delivery check, preserving captions for owner review. Both renders of the eight original HEVC videos pass at 2 GiB (peak 1.55 GiB); the synthetic release gate remains green at 1 GiB. Run release-smoke for later changes to compiler, verifier, renderer or admission; no implementation-ready todo remains.
- T110 remains owner-blocked: privately load a 9:16 clip with both cards in the Naver picker without publishing, capture the clip-tab overlays and supply the images for CDS-11 measurement. Remaining CDS interpretations are recorded in the results of T107, T108, T109 and T115 (contrast/card geometry, audio seams, exposure and frequency, footage-bound duration).
- T008 needs the owner present: refresh PUB@4 / ARCH@2, rerun postpilot-agent setup before installation for driver signature smarteditor-one-20260910-a6, and cancel or deliberately reuse the queued 20260905-test job; after completion, update PUB for VIDEO-17 and TMPL-39.

## log
- 260912 T117 done (ovl); overlap no longer blocks generated or manual clips, including furniture and final verification; both 30 s renders of the eight originals pass, external model cost $0; production media and release smokes and local CI checks pass
- 260912 T117 claimed (ovl)
- 260912 create-task CDS done (ovl); r4 → T117, overlap advisory throughout delivery
- 260912 create-task CDS start (ovl)
- 260912 update-ssot CDS done (ovl); r4 makes overlap advisory throughout delivery, including furniture and manual plans; other checks remain enforced
- 260912 update-ssot CDS start (ovl); overlap becomes advisory in automatic generation and manual rerendering
- 260912 clip overlap hotfix start (ovl); investigate caption collision failures and verify non-blocking delivery against the eight local source videos
- 260912 T116 done (fbl); a compiled plan whose manifest fails a check walks CDS-55's ladder before any download — style → 깔끔하게, anchor → the style's default, drop — at both pre-download verify points (`Render` and `Layout`, which now returns the repaired plan the composer stores); the verifier names the failing caption's slot (`design.Failure`, `clip.LayoutError`), furniture fails at once, a person's plan is refused untouched, the post-sample contrast fallback is rung 1 of the same ladder, every rung is recorded on `Composition.Fallback`, and the post-render verify stays hard (RENDER.md)
- 260912 T115 done (fbl); CDS-37 is a target: `design.CutBounds(scene, preset)` reads the preset's range, `holdCutLengths` never refuses and `plan_cut_length` is gone, the reconciliation runs a target pass then a footage/fade-floor pass so a reachable timeline is never refused; the recorded single take compiles again. The release gate is GREEN again: the renderer's per-cut source requests are served from one held download (`renderLoader`), the timing fixture reads the 음식점 preset's ends, the workspace bound counts T107's card measurement, and the speech probes map through the persisted plan
- 260912 T112 done (fbl); the clip page reads T111's live eligibility: every registered observe model stays listed, only `eligible` is pickable/quotable, a refused saved choice stays selected with its one CLIP-44 reason, loading/failed close the action with a retry, and the quote is bound to the status it was read under; the reusable selector gained an optional `availability` verdict (in `entities/model-catalog`) and lost `requireInlineStaticVideo`
- 260912 T111 done (fbl); clip-analysis admission is vendor-neutral: the catalog's `video_input` is the only pre-read gate, every leaf of the model's current OpenRouter endpoint document is qualified route → parameters → price and the cheapest qualified leaf is frozen (`CallPricing` v2 carries `Endpoint` + `RequiredParameters`, so pre-deploy quotes/jobs re-quote), `ListClipAnalysisEligibility` answers CLIP-44 with the four `CLIP_MODEL_*` reasons, and the recheck before every completion refuses drift with no retry or substitute; `InlineStaticVideo` survives only as the `processing: static` profile — the web picker still keys on it until T112
- 260912 hotfix clip generation (fable); every live clip since T105 failed at `plan` with a bare `MODEL_OUTPUT_INVALID`: the plan prompt offered 상호 as a chip while CDS-30's vocabulary has five labels (parser refused `plan_chip_label`), a template approving only 깔끔하게 tripped V14's run rule the composer could not avoid (`plan_layout_frequency`), a 30 s target over 36 s of food takes was unreachable under CDS-37 (`plan_timeline`), and the worker's log allowlist did not name any code T104–T109 added. Now the prompt reads `design.Fact.Chips`, unknown chips are dropped, the verifier reads the template's approved styles (`EditPlan.Styles`, `design.VerifyApproved`) and exempts a run no approved alternate could break, a cut may grow across touching observed segments, and a footage-bound clip ships at the length the footage holds when the 15 s floor accepts it (the target is only held from above). Verified live: 8 sources, 29.3 s 1080×1920 delivered, 4 runs $0.048
- 260912 create-task CDS done (trs); CDS r3 → T115 cut length becomes a target the compiler aims at (deletes `plan_cut_length`, reads the preset range as the target, restores the release-smoke fixture T109 had to raise) · T116 the CDS-55 repair ladder (style→anchor→drop on a compiled plan, furniture still fails at once, a person's plan still refused rather than moved); CDS tasked=3; both are rule changes inside packages that already hold what they need — no new port, no contract change
- 260912 update-ssot CDS done (trs); CDS r3 takes the cut range back OFF the hard-gate path — 1.2–6.0 s (food 4.0 s, the preset's own where it names one) is a TARGET the compiler aims at and never refuses for, the fade share is one flat 40 % of the boundaries again, and CDS-52's "reapply the fallback rules" is now implementable: CDS-55 names the ladder (style→깔끔하게 → anchor→default → drop the copy) and only the design system's OWN furniture failing a check fails the render; r2's preset delegation never reached code, so this is a documentation revert plus one new decision
- 260912 update-ssot CDS start (trs)
- 260912 create-task CDS start (trs)
- 260912 update-ssot CDS done (trs); CDS r2 delegates the cut range and the fade ratio to the template's category preset — 1.2–8.0 s is now only a guardrail, 2.5–6.0 s covers a template with no preset, a food close-up stays ≤ 4.0 s whatever the preset, and the fade share is 20 % cut-led / 60 % fade-led of the BOUNDARIES; CDS-50's per-preset ranges now bind every cut rather than only its close-ups, interiors or scenery; no doing task is affected (T110 is blocked on the owner and touches only `safe`)
- 260912 update-ssot CDS start (trs)
- 260912 T110 blocked (trs); every item needs the owner in front of the Naver app — the four overlay bounds cannot be measured from this machine and a guessed constant is the one thing CDS-10 forbids; what the owner has to do is listed in the task's result, and the Docker media smoke is already green so only the phone's part is missing
- 260912 T107 archived (trs); the `cds` session logged it done and removed its STATE row but left the file at `doing` with "the production image completes the smoke as nonroot" unticked, which failed the spec lint — the media-smoke stage was built and run for T108 and passes, so that item is ticked and the file is in done/
