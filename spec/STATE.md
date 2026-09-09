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
| QUOTA | 6 | 6 | - | 0 |
| POST | 5 | 5 | - | 0 |
| VOICE | 2 | 2 | - | 1 |
| GEN | 5 | 5 | - | 0 |
| MODEL | 8 | 8 | - | 0 |
| TEMPLATE | 4 | 4 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUBLISH | 4 | 4 | - | 0 |
| LANG | 2 | 2 | - | 0 |
| THEME | 8 | 8 | - | 0 |
| MARKETING | 4 | 4 | - | 0 |
| VIDEO | 1 | 1 | - | 1 |
| CLIP | 1 | 0 | all | 0 |
| BILLING | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 T046 | todo |
| T042 | The r4 mutation vocabulary and the body mutations | PUBLISH | T018 | blocked@260908 |
| T043 | Caret-relative image insertion, one-at-a-time upload and captions | PUBLISH | T042 | todo |
| T045 | The commit fence: arming, one activation, and readback through the post-view URL | PUBLISH | T044 T043 | todo |
| T046 | Wiring the real publisher into the daemon | PUBLISH | T045 | todo |
| T068 | `/posts` narrows by a title/tag search and a status filter | POST | T066 T067 | todo |

## next
- create-task CLIP from CLIP@1 (independent clip projects, video templates, AI cut planning, correction, transient sources and downloadable Naver-ready results)
- implement-task T068 (its two deps are done: the list answer carries tags, and the dock is `dock="list"` at every width)
- T067 changes the dock on `/voices` and `/templates` too, since THEME-24 is design language; that was the owner's accepted reading at update-ssot
- the paste protocol shipped end to end (T064 BE + T065 FE) and MODEL@8 is fully tasked; the next code work is the PUBLISH chain below
- T063 shipped the tag count option end to end (BE + FE gates green); next code work is the PUBLISH chain below
- update-ssot VOICE: VOICE-42✎ (r2) named a frozen tag count, but the rule comparison prompt emits prose and never asks for tags — drop the words or say what they would change
- Google sign-in is live end to end as of 260909 (AUTH-43): OAuth client registered, `GOOGLE_CLIENT_ID/SECRET` on the VPS `.env`, `VITE_GOOGLE_CLIENT_ID` in the Cloudflare build, button visible on `/login` and `/signup`, and the owner's real Google login succeeded
- the template data fields are done end to end (T055-T059): `pnpm --filter ./frontend test`, the Go suite, both lints and the build are green, and the four commits are on main
- also owed (THEME@6 · MARKETING@4 were implemented directly by the session that revised them, so no create-task is owed there, and THEME-29 already carries the `Switch` T058 needs — T058 re-stamps its base to THEME@6 at claim): BEFORE T058, TEMPLATE-41 excludes `label` from the format guide while TEMPLATE-43's `ask` requires that attribute (`guide.test.ts` asserts the guide holds no `label=`), so update-ssot TEMPLATE must name the retired SLOT label specifically or rename the attribute — owner's call; and `haeram-spec-creator lint` still rejects PUBLISH BILLING TEMPLATE MARKETING against FORMAT's `2-6 uppercase` id rule
- the PUBLISH chain stays as it was: unblock T042 with ONE live survey pass on a clean writer draft (does 문단 서식 변경 convert the caret's paragraph or its whole component on a multi-paragraph component, same for 인용구, what Enter from a converted block opens, how the list toolbar behaves there — the owner must discard the leftover dirty draft first), then T043 → T045 → T046 → T008, whose base must be re-read at PUBLISH@4 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes
## log
- 260909 T067 done (the dock change also landed on /voices and /templates)
- 260909 T067 claimed (dock)
- 260909 T066 done
- 260909 T066 claimed (tag)
- 260909 create-task CLIP start
- 260909 create-task T066 T067 T068 from POST@5 THEME@8 (BE `PostSummary.tags` · the list dock at every width incl. /voices and /templates · the FE narrowing); T068 deps on T066 for the field and on T067 for the shared PostsPage.tsx
- 260909 create-ssot CLIP r1 (CLIP-1+ … 28+: independent projects, video templates, AI-selected cuts, correction, transient sources, result-only retention and no direct publishing) — create-task CLIP owed
- 260909 create-task POST THEME start
- 260909 update-ssot POST r5 THEME r8 (POST-43✎ 64+ 65+ 66+ 67+ 68+ 69+ · THEME-24✎ a list's add dock is no longer phone-only); no doing task in scope (T065 is MODEL) — create-task POST THEME owed
- 260909 update-ssot POST THEME start (post list: 새 글 as a dock at every width, filter + search by title/tag)
- 260909 create-ssot CLIP start
- 260909 T065 claimed (blk)
- 260909 update-ssot MODEL r8 (MODEL-53✎); no doing task in scope, T065 (todo) unaffected — its ssot is MODEL-52 54 55 56
- 260909 update-ssot MODEL start (MODEL-53 reject causes vs what the context can see)
- 260909 T064 done
- 260909 T064 claimed (cdp)
- 260909 create-task T064 T065 from MODEL@7 (BE / FE split; live catalog read is mandatory for preview and apply)
- 260909 create-task MODEL start
- 260909 update-ssot MODEL r7 (MODEL-51+ 52+ 53+ 54+ 55+ 56+); no doing task in MODEL scope — create-task MODEL owed
- 260909 update-ssot MODEL start (bulk list paste → parsed registration update on 모델 관리)
