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
| T107 | Hook and ending cards, the Paperlogy face and brightness-aware scrims | CDS CLIP | - | todo |
| T108 | Conditional transitions and loudness normalisation | CDS CLIP | - | todo |
| T109 | Two sequential copies on a long cut | CDS CLIP | T108 | todo |
| T110 | Release QA on the Naver app and the overlay measurement | CDS CLIP | T107 T108 | todo |
| T111 | Vendor-neutral clip-analysis qualification and strict routing | CLIP QUOTA VIDEO MODEL LANG ARCH | - | todo |
| T112 | Clip-analysis model eligibility picker | CLIP LANG ARCH | T111 | todo |

## next
- the per-template 목표 글자 수·태그 수 shipped (T113 T114): a template seeds the post's options at assignment and the post keeps whatever the user types over them
- implement-task T107 (cards, Paperlogy, scrims) and T108 (transitions, loudness) in either order, then T109; T110 needs the owner with the Naver app and closes CDS-11. T111 → T112 are independent of this chain
- T102–T106 landed the design package, the renderer, the clip furniture, the composer and the surfaces: every following CDS task reads `backend/internal/clip/design` and its byte-identical mirror `frontend/src/shared/config/clip-design.json`, never a literal. Every render now carries its disclosure badge and its chips, the verifier gates each one (V1 V2 V5 V6 V7 V9 V10 V13 V14) and the generation gate refuses a clip with no campaign type or fewer than two facts. **A generation job queued before this deploy fails with CLIP_INVALID_INPUT and must be re-approved** (payload version 3). **The model no longer chooses style, position or accent** — it writes words, a short alternative, a keyword, a hook and each cut's chips, and the CDS tables place them. The frontend now reads `shared/config/clip-design.json` itself, so no clip number is typed by hand on either side. V3 contrast waits for T107's brightness sampler
- T008 needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job; once it closes, update-ssot PUB for VIDEO-17 + TMPL-39
## log
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
- 260911 T105 claimed (cds)
- 260911 T104 done (cds); disclosure badge + information chips on every render, the five CDS-50 presets and the reserved facts in `design.json`, an approval gate that refuses a clip with no campaign type or fewer than two of 상호·위치·가격·메뉴, and 10 new `CLIP_*` reasons; the generation payload went to version 3 (one named constant, three readers were checking 2) so a QUEUED JOB MUST BE RE-APPROVED; the badge got its own static overlay layer because it may not move while the copy must; a long chip value is cut with an ellipsis rather than squeezed
- 260911 T104 claimed (cds)
- 260911 T103 done (cds); the four styles, the 180/120 ms motions and a pre-FFmpeg manifest verifier (V1 V2 V5 V7 V9 V13 V14) with six `CLIP_LAYOUT_*` reasons; the bundled FFmpeg had no `fade` filter — its allowlist now carries one, because CDS-4 admits no other entrance; `design.Verify` takes the manifest and a ratio, not `clip` types, which would be an import cycle; the keyword's offset is measured through the keyword because a prefix can be a space with no ink box
- 260911 T103 claimed (cds)
- 260911 T102 done (cds); one embedded `design.json` (+ byte-identical FE mirror) pins every CDS constant, `Caption` speaks anchors/alignments and the four style ids, migration 0041 and a read-time token belt carry every stored plan and template over, and `ValidateEditPlan` enforces the per-style line/char limits and CDS-41 exposure; 16:9/1:1 LEFT·RIGHT·CENTER and their scrim rectangles are derived from CDS-47/48's stated numbers (in the task result), and CDS-23's "26 total" vs 2 × 14 wants an update-ssot
- 260911 create-task CLIP done (prov); CLIP r6 → T111 vendor-neutral backend qualification/routing · T112 clip picker/reasons; CLIP tasked=6; T111 is independent of T102
- 260911 update-ssot TMPL POST start (len)
- 260911 create-task CLIP start (prov)
- 260911 update-ssot CLIP done (prov); r6 removes provider/model-family admission allowlists, qualifies every registered video-input observer by current inline endpoint/request/price compatibility, and gives ineligible models a stable reason; T102 is unaffected
