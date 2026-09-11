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
| CDS | 1 | 1 | - | 2 |
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
| T111 | Vendor-neutral clip-analysis qualification and strict routing | CLIP QUOTA VIDEO MODEL LANG ARCH | - | todo |
| T112 | Clip-analysis model eligibility picker | CLIP LANG ARCH | T111 | todo |

## next
- the per-template 목표 글자 수·태그 수 shipped (T113 T114): a template seeds the post's options at assignment and the post keeps whatever the user types over them
- T108 shipped CDS-36 and CDS-35: every cut carries its own `TransitionMS` (0 hard cut · 200 fade · 300 fade-through-black, first cut always 0), the duration is `Σ lengths − Σ transitions` everywhere, and the compiler joins two cuts by whether their SCENE differs, capped at 40 % of the boundaries. **CDS-37 is now enforced (1.2–6.0 s, food ≤ 4.0 s), so a clip has at least three cuts and 90 s holds at most 75** — a single-take plan is refused as `plan_timeline` and a cut with no room to reach 1.2 s as `plan_cut_length`. Audio is assembled once, measured, then loudnorm-corrected inside the one final encode, and V12 is read off the DELIVERED file. **A plan stored before this deploy reads back with a fade at every boundary** (stored version 2, versions 0–1 backfilled). Two CDS readings in T108's result want an update-ssot: what a 60 ms cross-fade means at a hard cut, and that CDS-37 fixes a minimum cut count
- T109 shipped CDS-43: a cut carries `Copies`, one or two, and every per-caption rule — the style limits, the exposure, CDS-38's step, CDS-40's frequency — is read on the COPY, not the cut. A cut of 4 s or more whose sentence describes also states the number it leads to, 120 ms later; the compiler lifts it from the `short_text` the model already wrote, so no extra call. **A plan stored before this deploy reads back as the one copy it was** (stored version 3), and `ClipEditCut.copy` stays populated for one release beside `copies`. Three readings want an update-ssot, in T109's result: V4's 60 % occupancy contradicts CDS-45 and is not enforced; a `NUM` clause of the caption can almost never be the second copy because CDS-39 reads numbers first; CDS-40's guards now count copies
- T110 is blocked on the owner and nothing else: render a 9:16 clip with both cards, upload it privately through the Naver Clip picker WITHOUT publishing, screenshot the clip tab, and send the images back — normalising, measuring and writing `docs/design/naver-clip-overlay-measurement-YYMMDD.md` is mechanical from there, and CDS-11 closes with an update-ssot. T111 → T112 are independent of this chain and are the next implementable work
- T102–T106 landed the design package, the renderer, the clip furniture, the composer and the surfaces: every following CDS task reads `backend/internal/clip/design` and its byte-identical mirror `frontend/src/shared/config/clip-design.json`, never a literal. Every render now carries its disclosure badge and its chips, the verifier gates each one (V1 V2 V5 V6 V7 V9 V10 V13 V14) and the generation gate refuses a clip with no campaign type or fewer than two facts. **A generation job queued before this deploy fails with CLIP_INVALID_INPUT and must be re-approved** (payload version 3). **The model no longer chooses style, position or accent** — it writes words, a short alternative, a keyword, a hook and each cut's chips, and the CDS tables place them. The frontend now reads `shared/config/clip-design.json` itself, so no clip number is typed by hand on either side
- T107 added the two cards, the Paperlogy face and the CDS-44 sampler: a clip now opens on a hook card and closes on a CTA card, `ClipEditPlan.hook` is stored and editable on step ②, and V3 contrast is live (`CLIP_LAYOUT_CONTRAST`). The copy plate is rasterized inside the cut's own source callback, so anything that needs the footage under a caption belongs there; `composed` carries the plan, the layers, the grounds and the manifest through that pass. **Two CDS readings are recorded in the task result and want an update-ssot**: what surface V3 measures an unplated style against, and the hook card's 16 px overhang past the 9:16 safe area
- T008 needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job; once it closes, update-ssot PUB for VIDEO-17 + TMPL-39
## log
- 260912 T110 blocked (trs); every item needs the owner in front of the Naver app — the four overlay bounds cannot be measured from this machine and a guessed constant is the one thing CDS-10 forbids; what the owner has to do is listed in the task's result, and the Docker media smoke is already green so only the phone's part is missing
- 260912 T107 archived (trs); the `cds` session logged it done and removed its STATE row but left the file at `doing` with "the production image completes the smoke as nonroot" unticked, which failed the spec lint — the media-smoke stage was built and run for T108 and passes, so that item is ticked and the file is in done/
- 260912 T110 claimed (trs)
- 260912 T109 done (trs); a cut carries `Copies` and a manifest element names which one it belongs to, so the verifier keys every per-caption rule by (cut, copy) rather than by cut; CDS-39 reads a number FIRST, which means a `DESC` sentence can never contain one and the second copy is in practice the model's own `short_text`; V4's 60 % occupancy is NOT enforced because the manifest carries no cut length and CDS-45 makes it unreachable anyway (the smoke's own last cut ends its copy at 44 % so nothing shows under the ending card); the release-smoke fixtures were a 15 s single take and a 1000 ms cut, both refused by T108's CDS-37, and are migrated here
- 260911 T109 claimed (trs)
- 260911 T108 done (trs); every cut carries the transition INTO it and the duration is the footage less those overlaps — a hard cut is a `concat`, a scene change an `xfade`, and the 40 % cap keeps the EARLIEST fades; CDS-37's lengths are held in the compiler, which makes a single-take clip impossible and caps 90 s at 75 cuts (the recorded live single-take response is now refused, and the test says so); CDS-35's 60 ms at a hard cut is an `afade` pair either side of the seam, not an `acrossfade`, because an acrossfade consumes 60 ms the picture does not and drifts every later cut; loudness is measured on the assembled track (FFmpeg's stderr, a first in this codebase) and corrected inside the one final encode; V12 reads the delivered file
- 260911 T108 claimed (trs)
- 260911 T107 done (cds); both cards, the Paperlogy face and CDS-44's sampler ship — the copy plate is now rasterized INSIDE the cut's own source callback so an unplated style can read the footage under it, and the manifest is verified twice (geometry before any download, the sampled grounds and any fallback before the cuts are joined); V3 reads an unplated text against its own `stroke.dark` outline over the scrim-washed ground, because the bare footage fails 4.5:1 above L≈0.18 and would retire 크게 강조 and 형광펜 on all daylight footage (two update-ssot CDS proposals in the task result, with the card's 16 px safe-area overhang); the card layer's filter graph was broken before its golden existed
- 260911 review: `release-smoke` had been red since T104 with THREE breakages its recorded fixtures never saw — no campaign type (T104's gate), an observation missing `scene`/`readable_text`/`subject` (T105) and a plan still naming style/anchor per caption (T105). All fixed in T107 because it is the only end-to-end proof of the generation path. `go test ./...` cannot see it: the test is gated on `CLIP_RELEASE_SMOKE=1`, so a `release-smoke` run belongs in the acceptance of every task that changes a model contract
- 260911 T107 claimed (cds)
- 260911 T106 done (cds); the FE reads the design mirror through a typed `shared/config/clip-design`, the preview draws the four styles from it and a test pins the palette against it; preset picker + `SeedPresetFields` merge with a dialog when clips already use the template, a required campaign type and a CTA on step ①, anchor+align+keyword+chips on step ②, and the FE validator mirrors every per-style, exposure, step and frequency rule; a `CLIP_LAYOUT_*` reason names the CHECK not the cut, so it shows in the step's status region while the FE mirror speaks beside each field
- 260911 T114 done (len); the template screen authors both numbers behind their own 사용 tick inside the one draft, and the picker's own SavePostDraft response is what shows the seeded values — `applyingSavedDraft` takes them only when the template id changed, so a title autosave cannot roll back an options save; FE gate green except three clip test files and two clip style escapes owned by the parallel CDS session
- 260911 T106 claimed (cds)
- 260911 T105 done (cds); CDS-39/40/38/41/42 are data-driven pure functions and the model writes words only; CDS-40's alternation and CDS-38's one-step rule contradict each other, so the step is a preference and V13 measures it only within one style; `short_text` also covers a style's character limit; the cut extension may only use the target's remaining slack; a dropped copy carries no placement; the shape checker had no boolean case; the three recorded fixtures were migrated mechanically, not re-recorded
- 260911 T114 claimed (len)
- 260911 T113 done (len); a template carries optional `target_length`/`tag_count` (migration 0043, NULL = no opinion) and an assignment seeds the post's own options with `COALESCE` in the assigning statement; a run still freezes the POST's values; the length has a floor and no ceiling because the post's option has none; sqlc rewrites parameters by byte offset, so a non-ASCII character anywhere in a queries/*.sql file breaks that whole file
- 260911 T113 claimed (len)
- 260911 create-task TMPL POST done (len); TMPL r6 + POST r6 → T113 contract·migration·validation·seed in the assigning statement · T114 template screen fields and the seeded values on the post; TMPL tasked=6, POST tasked=6; the seed reuses the post's own bounds and the directory port already in place, so no new config key and no new port
- 260911 create-task TMPL POST start (len)
- 260911 update-ssot TMPL POST done (len); TMPL r6 gives a template two optional numbers (`target_length` `tag_count`, unset = 의견 없음) that seed the post's own options on assignment and reach no prompt, POST r6 puts the seed inside the assigning transaction; TMPL-34's per-template length/tags rejection lifted; no doing task is affected and GEN is untouched (a run still freezes the post's values)
