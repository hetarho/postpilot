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
| POST | 4 | 4 | - | 0 |
| VOICE | 2 | 2 | - | 1 |
| GEN | 5 | 5 | - | 0 |
| MODEL | 8 | 7 | MODEL-53✎ | 0 |
| TEMPLATE | 4 | 4 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUBLISH | 4 | 4 | - | 0 |
| LANG | 2 | 2 | - | 0 |
| THEME | 7 | 7 | - | 0 |
| MARKETING | 4 | 4 | - | 0 |
| VIDEO | 1 | 1 | - | 1 |
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

## next
- create-task MODEL from MODEL@8 — expected no-op: MODEL-53✎ only makes the SSOT say what T064 (done) already implements; the paste protocol shipped end to end (T064 BE + T065 FE), so the next code work is the PUBLISH chain below
- T063 shipped the tag count option end to end (BE + FE gates green); next code work is the PUBLISH chain below
- update-ssot VOICE: VOICE-42✎ (r2) named a frozen tag count, but the rule comparison prompt emits prose and never asks for tags — drop the words or say what they would change
- Google sign-in is live end to end as of 260909 (AUTH-43): OAuth client registered, `GOOGLE_CLIENT_ID/SECRET` on the VPS `.env`, `VITE_GOOGLE_CLIENT_ID` in the Cloudflare build, button visible on `/login` and `/signup`, and the owner's real Google login succeeded
- the template data fields are done end to end (T055-T059): `pnpm --filter ./frontend test`, the Go suite, both lints and the build are green, and the four commits are on main
- also owed (THEME@6 · MARKETING@4 were implemented directly by the session that revised them, so no create-task is owed there, and THEME-29 already carries the `Switch` T058 needs — T058 re-stamps its base to THEME@6 at claim): BEFORE T058, TEMPLATE-41 excludes `label` from the format guide while TEMPLATE-43's `ask` requires that attribute (`guide.test.ts` asserts the guide holds no `label=`), so update-ssot TEMPLATE must name the retired SLOT label specifically or rename the attribute — owner's call; and `haeram-spec-creator lint` still rejects PUBLISH BILLING TEMPLATE MARKETING against FORMAT's `2-6 uppercase` id rule
- the PUBLISH chain stays as it was: unblock T042 with ONE live survey pass on a clean writer draft (does 문단 서식 변경 convert the caret's paragraph or its whole component on a multi-paragraph component, same for 인용구, what Enter from a converted block opens, how the list toolbar behaves there — the owner must discard the leftover dirty draft first), then T043 → T045 → T046 → T008, whose base must be re-read at PUBLISH@4 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes
## log
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
- 260909 T063 done
- 260909 T063 claimed (tgc)
- 260909 AUTH-43 ops done: Google OAuth client + env on both ends, button live on /login and /signup
- 260909 create-task T063 from POST@4 GEN@5 LANG@2 MODEL@6; VOICE r2 no-op (no code impact: the rule comparison prompt asks for no tags)
- 260909 T062 done
- 260909 T062 claimed (srt)
- 260909 create-task POST GEN LANG MODEL VOICE start (tag_count)
- 260909 T061 done
- 260909 update-ssot MODEL r5 (MODEL-20✎ effort goes with the registration) from T061 — implemented in T061, tasked 4→5, no create-task owed; T062 unaffected
