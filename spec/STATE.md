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
| POST | 5 | 5 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 6 | 6 | - | 0 |
| MODEL | 9 | 9 | - | 0 |
| TMPL | 5 | 5 | - | 1 |
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
| T104 | Disclosure badge, information chips, category presets and the project's facts | CDS CLIP LANG | - | todo |
| T105 | Deterministic composition: scene-aware analysis, sentence classes, style and anchor selection, exposure and grounding | CDS CLIP | T104 | todo |
| T106 | The clip surfaces speak the design system: four styles, anchors, presets, disclosure and CTA | CDS CLIP THEME LANG | T104 | todo |
| T107 | Hook and ending cards, the Paperlogy face and brightness-aware scrims | CDS CLIP | T105 T106 | todo |
| T108 | Conditional transitions and loudness normalisation | CDS CLIP | T105 T106 | todo |
| T109 | Two sequential copies on a long cut | CDS CLIP | T108 | todo |
| T110 | Release QA on the Naver app and the overlay measurement | CDS CLIP | T107 T108 | todo |
| T111 | Vendor-neutral clip-analysis qualification and strict routing | CLIP QUOTA VIDEO MODEL LANG ARCH | - | todo |
| T112 | Clip-analysis model eligibility picker | CLIP LANG ARCH | T111 | todo |

## next
- implement-task T104 (badge, chips, presets, facts), then T105 → T106 → T107 · T108 → T109; T110 needs the owner with the Naver app and closes CDS-11. T111 → T112 are independent of this chain
- T102/T103 landed the design package and the renderer: every following CDS task reads `backend/internal/clip/design` and its byte-identical mirror `frontend/src/shared/config/clip-design.json`, never a literal. The renderer now draws the four styles, the two motions and a manifest the verifier gates each render on; the clip surfaces still have no preset, alignment control, disclosure or CTA (T106's), and V3 contrast waits for T107's brightness sampler
- T008 needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job; once it closes, update-ssot PUB for VIDEO-17 + TMPL-39
## log
- 260911 T103 done (cds); the four styles, the 180/120 ms motions and a pre-FFmpeg manifest verifier (V1 V2 V5 V7 V9 V13 V14) with six `CLIP_LAYOUT_*` reasons; the bundled FFmpeg had no `fade` filter — its allowlist now carries one, because CDS-4 admits no other entrance; `design.Verify` takes the manifest and a ratio, not `clip` types, which would be an import cycle; the keyword's offset is measured through the keyword because a prefix can be a space with no ink box
- 260911 T103 claimed (cds)
- 260911 T102 done (cds); one embedded `design.json` (+ byte-identical FE mirror) pins every CDS constant, `Caption` speaks anchors/alignments and the four style ids, migration 0041 and a read-time token belt carry every stored plan and template over, and `ValidateEditPlan` enforces the per-style line/char limits and CDS-41 exposure; 16:9/1:1 LEFT·RIGHT·CENTER and their scrim rectangles are derived from CDS-47/48's stated numbers (in the task result), and CDS-23's "26 total" vs 2 × 14 wants an update-ssot
- 260911 create-task CLIP done (prov); CLIP r6 → T111 vendor-neutral backend qualification/routing · T112 clip picker/reasons; CLIP tasked=6; T111 is independent of T102
- 260911 update-ssot TMPL POST start (len)
- 260911 create-task CLIP start (prov)
- 260911 update-ssot CLIP done (prov); r6 removes provider/model-family admission allowlists, qualifies every registered video-input observer by current inline endpoint/request/price compatibility, and gives ineligible models a stable reason; T102 is unaffected
- 260911 update-ssot CLIP start (prov)
- 260911 T102 claimed (cds)
- 260911 create-task CDS done (dsgn); CDS r1 + CLIP r5 → T102 vocabulary/constants · T103 styles/motion/verifier · T104 badge/chips/presets/facts · T105 deterministic composition · T106 FE · T107 cards/Paperlogy/scrims · T108 transitions/loudness · T109 two copies · T110 Naver QA; CDS tasked=1, CLIP tasked=5; CDS-41's regenerate-twice is read as an in-call short_text alternative (no unplanned paid call) — owner to confirm or return via update-ssot
- 260911 create-task CDS start (dsgn); CDS r1 (all) + CLIP r5 delta CLIP-4✎ 13✎ 14✎ 15✎ 16✎ 18✎ 28✎; T101 shares ClipTemplateEditor with the preset/style task
- 260911 T101 done (tpl); 영상 템플릿 목록·상세가 글 템플릿 화면 모양으로, 삭제는 행으로 옮기며 연결 해제 수를 삭제 전에 경고; `clipDetachedCount` 라우터 상태 제거; locale parity 테스트가 en 누락을 잡음
- 260911 T101 claimed (tpl); the dsgn session confirmed no CDS code task exists and is touching no code, so this takes `ClipTemplateEditor.tsx` first
- 260911 T100 done (dir); `/clips` takes the post list's rows, badge, relative time and URL-carried search/filter; `ListControls` lifted into `shared/ui` and `filter-posts` rewired onto it unchanged; the badge reads the job first, the filter reads the state only
- 260911 T100 claimed (dir)
- 260911 T099 done (list); the list answer fills `latest_job` per project through the same port the detail uses, detail-only fields untouched; test in `clip/store`'s harness because `h.jobs` is a concrete queue, not a port; BE gate green, no proto change
- 260911 T099 claimed (list)
- 260911 T098 done (auto); module-level per-project settings queue (debounce, latest-wins, backoff, no retry on a repeatable refusal), 저장 버튼·dirty 게이트·이탈 다이얼로그 제거, `클립 만들기`만 `/clips/new`에 남음; `useSaveStatus`는 ARCH-18 때문에 shared로 못 가고 순수 부분만 `shared/lib/save-state`로
- 260911 T098 claimed (auto)
- 260911 T097 done (wksp); three steps from durable state, editor top row, one status region with CLIP-38's precedence, one dock per step, delete as its own slice; the bar follows the project's state and a failed attempt opens on its retry step, and the unsaved-correction guard moved up to the page; full FE gate green
