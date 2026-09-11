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
| CDS | 3 | 3 | - | 2 |
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
| T116 | The caption repair ladder | CDS CLIP | - | doing@260912.fbl |

## next
- two CDS readings from the clip hotfix want an update-ssot: a target the footage cannot reach under CDS-37 now ships SHORTER (floor 15 s) rather than refusing, and CDS-40's run rule is read against the template's approved set (a 깔끔하게-only template may run). T115/T116 touch the same compiler and verifier: read `timeline.go` `observedSpan` and `verify.go` `canAlternate` before changing either
- the per-template 목표 글자 수·태그 수 shipped (T113 T114): a template seeds the post's options at assignment and the post keeps whatever the user types over them
- T108 shipped CDS-36 and CDS-35: every cut carries its own `TransitionMS` (0 hard cut · 200 fade · 300 fade-through-black, first cut always 0), the duration is `Σ lengths − Σ transitions` everywhere, and the compiler joins two cuts by whether their SCENE differs, capped at 40 % of the boundaries. **CDS-37 is now enforced (1.2–6.0 s, food ≤ 4.0 s), so a clip has at least three cuts and 90 s holds at most 75** — a single-take plan is refused as `plan_timeline` and a cut with no room to reach 1.2 s as `plan_cut_length`. Audio is assembled once, measured, then loudnorm-corrected inside the one final encode, and V12 is read off the DELIVERED file. **A plan stored before this deploy reads back with a fade at every boundary** (stored version 2, versions 0–1 backfilled). Two CDS readings in T108's result want an update-ssot: what a 60 ms cross-fade means at a hard cut, and that CDS-37 fixes a minimum cut count
- T109 shipped CDS-43: a cut carries `Copies`, one or two, and every per-caption rule — the style limits, the exposure, CDS-38's step, CDS-40's frequency — is read on the COPY, not the cut. A cut of 4 s or more whose sentence describes also states the number it leads to, 120 ms later; the compiler lifts it from the `short_text` the model already wrote, so no extra call. **A plan stored before this deploy reads back as the one copy it was** (stored version 3), and `ClipEditCut.copy` stays populated for one release beside `copies`. Three readings want an update-ssot, in T109's result: V4's 60 % occupancy contradicts CDS-45 and is not enforced; a `NUM` clause of the caption can almost never be the second copy because CDS-39 reads numbers first; CDS-40's guards now count copies
- T116 is the last open compiler/verifier task; after it the clip chain has no failure path left for a caption — run `release-smoke` (see the T115 result for how) before `done` on anything touching the compiler, verifier, renderer or admission
- T110 is blocked on the owner and nothing else: render a 9:16 clip with both cards, upload it privately through the Naver Clip picker WITHOUT publishing, screenshot the clip tab, and send the images back — normalising, measuring and writing `docs/design/naver-clip-overlay-measurement-YYMMDD.md` is mechanical from there, and CDS-11 closes with an update-ssot. T111 → T112 are independent of this chain and are the next implementable work
- T102–T106 landed the design package, the renderer, the clip furniture, the composer and the surfaces: every following CDS task reads `backend/internal/clip/design` and its byte-identical mirror `frontend/src/shared/config/clip-design.json`, never a literal. Every render now carries its disclosure badge and its chips, the verifier gates each one (V1 V2 V5 V6 V7 V9 V10 V13 V14) and the generation gate refuses a clip with no campaign type or fewer than two facts. **A generation job queued before this deploy fails with CLIP_INVALID_INPUT and must be re-approved** (payload version 3). **The model no longer chooses style, position or accent** — it writes words, a short alternative, a keyword, a hook and each cut's chips, and the CDS tables place them. The frontend now reads `shared/config/clip-design.json` itself, so no clip number is typed by hand on either side
- T107 added the two cards, the Paperlogy face and the CDS-44 sampler: a clip now opens on a hook card and closes on a CTA card, `ClipEditPlan.hook` is stored and editable on step ②, and V3 contrast is live (`CLIP_LAYOUT_CONTRAST`). The copy plate is rasterized inside the cut's own source callback, so anything that needs the footage under a caption belongs there; `composed` carries the plan, the layers, the grounds and the manifest through that pass. **Two CDS readings are recorded in the task result and want an update-ssot**: what surface V3 measures an unplated style against, and the hook card's 16 px overhang past the 9:16 safe area
- T008 needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job; once it closes, update-ssot PUB for VIDEO-17 + TMPL-39
## log
- 260912 T116 claimed (fbl)
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
- 260912 T110 claimed (trs)
- 260912 T109 done (trs); a cut carries `Copies` and a manifest element names which one it belongs to, so the verifier keys every per-caption rule by (cut, copy) rather than by cut; CDS-39 reads a number FIRST, which means a `DESC` sentence can never contain one and the second copy is in practice the model's own `short_text`; V4's 60 % occupancy is NOT enforced because the manifest carries no cut length and CDS-45 makes it unreachable anyway (the smoke's own last cut ends its copy at 44 % so nothing shows under the ending card); the release-smoke fixtures were a 15 s single take and a 1000 ms cut, both refused by T108's CDS-37, and are migrated here
- 260911 T109 claimed (trs)
- 260911 T108 done (trs); every cut carries the transition INTO it and the duration is the footage less those overlaps — a hard cut is a `concat`, a scene change an `xfade`, and the 40 % cap keeps the EARLIEST fades; CDS-37's lengths are held in the compiler, which makes a single-take clip impossible and caps 90 s at 75 cuts (the recorded live single-take response is now refused, and the test says so); CDS-35's 60 ms at a hard cut is an `afade` pair either side of the seam, not an `acrossfade`, because an acrossfade consumes 60 ms the picture does not and drifts every later cut; loudness is measured on the assembled track (FFmpeg's stderr, a first in this codebase) and corrected inside the one final encode; V12 reads the delivered file
- 260911 T108 claimed (trs)
- 260911 T107 done (cds); both cards, the Paperlogy face and CDS-44's sampler ship — the copy plate is now rasterized INSIDE the cut's own source callback so an unplated style can read the footage under it, and the manifest is verified twice (geometry before any download, the sampled grounds and any fallback before the cuts are joined); V3 reads an unplated text against its own `stroke.dark` outline over the scrim-washed ground, because the bare footage fails 4.5:1 above L≈0.18 and would retire 크게 강조 and 형광펜 on all daylight footage (two update-ssot CDS proposals in the task result, with the card's 16 px safe-area overhang); the card layer's filter graph was broken before its golden existed
- 260911 review: `release-smoke` had been red since T104 with THREE breakages its recorded fixtures never saw — no campaign type (T104's gate), an observation missing `scene`/`readable_text`/`subject` (T105) and a plan still naming style/anchor per caption (T105). All fixed in T107 because it is the only end-to-end proof of the generation path. `go test ./...` cannot see it: the test is gated on `CLIP_RELEASE_SMOKE=1`, so a `release-smoke` run belongs in the acceptance of every task that changes a model contract
