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
| CLIP | 4 | 4 | - | 0 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |

## next
- the CLIP r4 + THEME r10 unification is complete (T097–T101), so `ClipTemplateEditor` is free: the CDS task that adds the category preset and the four styles builds on its new shape rather than waiting on it
- T008 needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job; once it closes, update-ssot PUB for VIDEO-17 + TMPL-39
## log
- 260911 T101 done (tpl); 영상 템플릿 목록·상세가 글 템플릿 화면 모양으로, 삭제는 행으로 옮기며 연결 해제 수를 삭제 전에 경고; `clipDetachedCount` 라우터 상태 제거; locale parity 테스트가 en 누락을 잡음
- 260911 T101 claimed (tpl); the dsgn session confirmed no CDS code task exists and is touching no code, so this takes `ClipTemplateEditor.tsx` first
- 260911 T100 done (dir); `/clips` takes the post list's rows, badge, relative time and URL-carried search/filter; `ListControls` lifted into `shared/ui` and `filter-posts` rewired onto it unchanged; the badge reads the job first, the filter reads the state only
- 260911 T100 claimed (dir)
- 260911 T099 done (list); the list answer fills `latest_job` per project through the same port the detail uses, detail-only fields untouched; test in `clip/store`'s harness because `h.jobs` is a concrete queue, not a port; BE gate green, no proto change
- 260911 T099 claimed (list)
- 260911 T098 done (auto); module-level per-project settings queue (debounce, latest-wins, backoff, no retry on a repeatable refusal), 저장 버튼·dirty 게이트·이탈 다이얼로그 제거, `클립 만들기`만 `/clips/new`에 남음; `useSaveStatus`는 ARCH-18 때문에 shared로 못 가고 순수 부분만 `shared/lib/save-state`로
- 260911 T098 claimed (auto)
- 260911 T097 done (wksp); three steps from durable state, editor top row, one status region with CLIP-38's precedence, one dock per step, delete as its own slice; the bar follows the project's state and a failed attempt opens on its retry step, and the unsaved-correction guard moved up to the page; full FE gate green
- 260911 T097 claimed (wksp)
- 260911 create-task CLIP THEME merged (unif); a parallel session's CLIP r5 + CDS r1 landed mid-run in the shared tree, so T097..T101 were rebased to CLIP@5 and T101 took `ClipTemplateEditor` first; this commit carries the r4 delta only — the r5 ✎ lines and the CDS rows stay with that session
- 260911 create-task CLIP THEME done (unif); r4+r10 → T097 workspace shape · T098 settings autosave · T099 BE list latest job · T100 `/clips` rows and narrowing · T101 영상 템플릿 screens; CLIP tasked=4, THEME tasked=10; no proto change and CLIP-9's fixed ratio untouched
- 260911 create-task CLIP THEME start (unif); CLIP r4 delta CLIP-36+ 37+ 38+ 39+ 40+ 41+ 42+ CLIP-17✎ and THEME r10 THEME-39+
- 260911 update-ssot CLIP THEME done (unif); THEME-39 makes the lifecycle shape a product rule, CLIP r4 adopts it on all five clip surfaces; owner chose 3 steps, autosave, the full post list and the 영상 템플릿 screens in scope, with 클립 만들기 kept on `/clips/new` so CLIP-9's fixed ratio and the BE's ratio-less patch stand
- 260911 update-ssot CLIP THEME start (unif); the clip working surface and directory diverge from the post editor's step bar, one status region, one dock and row shape
- 260911 T096 done; original failure reproduced and fixed by bounded deterministic timeline compilation; real 15s MP4 and authenticated queue/storage/download verified; e850fa8 pushed, CI 34549381300 and deployment 34549381321 pass, exact production image healthy; reported paid total USD 0.020052
