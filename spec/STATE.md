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
| POST | 3 | 3 | - | 0 |
| VOICE | 1 | 1 | - | 1 |
| GEN | 4 | 4 | - | 0 |
| MODEL | 4 | 4 | - | 0 |
| TEMPLATE | 4 | 4 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUBLISH | 4 | 4 | - | 0 |
| LANG | 1 | 1 | - | 0 |
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
| T061 | Batch variants dropped at mapping, delisting deregisters and hides | MODEL | - | todo |
| T062 | Price sort on 모델 관리 and the retired delisted badge | MODEL | - | todo |
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 T046 | todo |
| T042 | The r4 mutation vocabulary and the body mutations | PUBLISH | T018 | blocked@260908 |
| T043 | Caret-relative image insertion, one-at-a-time upload and captions | PUBLISH | T042 | todo |
| T045 | The commit fence: arming, one activation, and readback through the post-view URL | PUBLISH | T044 T043 | todo |
| T046 | Wiring the real publisher into the daemon | PUBLISH | T045 | todo |

## next
- implement-task T061 then T062 (MODEL r4; independent, either order)
- AUTH-43 is ops, not code (T060 shipped the screens): the owner registers a Google OAuth web client (redirect `<app origin>/login/google/callback`), sets `GOOGLE_CLIENT_ID` + `GOOGLE_CLIENT_SECRET` in `/srv/postpilot-prod/.env` and `VITE_GOOGLE_CLIENT_ID` in the Cloudflare Worker build variables, then redeploys both
- the template data fields are done end to end (T055-T059): `pnpm --filter ./frontend test`, the Go suite, both lints and the build are green, and the four commits are on main
- also owed (THEME@6 · MARKETING@4 were implemented directly by the session that revised them, so no create-task is owed there, and THEME-29 already carries the `Switch` T058 needs — T058 re-stamps its base to THEME@6 at claim): BEFORE T058, TEMPLATE-41 excludes `label` from the format guide while TEMPLATE-43's `ask` requires that attribute (`guide.test.ts` asserts the guide holds no `label=`), so update-ssot TEMPLATE must name the retired SLOT label specifically or rename the attribute — owner's call; and `haeram-spec-creator lint` still rejects PUBLISH BILLING TEMPLATE MARKETING against FORMAT's `2-6 uppercase` id rule
- the PUBLISH chain stays as it was: unblock T042 with ONE live survey pass on a clean writer draft (does 문단 서식 변경 convert the caret's paragraph or its whole component on a multi-paragraph component, same for 인용구, what Enter from a converted block opens, how the list toolbar behaves there — the owner must discard the leftover dirty draft first), then T043 → T045 → T046 → T008, whose base must be re-read at PUBLISH@4 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes
## log
- 260909 T060 done
- 260909 create-task T061 T062 from MODEL@4
- 260909 T060 claimed (ttl)
- 260909 create-task MODEL start
- 260909 update-ssot MODEL r4 (MODEL-20✎ 22✎ 28✎ 50+); no doing task in scope
- 260909 update-ssot MODEL start (price sort, batch variants hidden, delisted registrations)
- 260909 create-task T060 from AUTH@5 (AUTH-43 has no code delta: env, gating and DEPLOY.md rows exist — owner ops)
- 260909 create-task AUTH start
- 260909 update-ssot AUTH r5 (AUTH-42✎ password confirmation · AUTH-45 ?→x no guest mode)
- 260909 update-ssot AUTH r4 (AUTH-30✎ 42+ 43+ 44x 45?) — QUOTA untouched until AUTH-45 is answered; no doing task in scope
- 260909 update-ssot AUTH QUOTA start (login/signup distinction, Google sign-in rollout, signed-out free tier)
- 260909 T059 done
- 260909 T059 claimed (fld)
- 260909 T058 done
- 260909 update-ssot THEME r7 (THEME-37✎ shader aurora, grain, pointer tilt) implemented directly — tasked 6→7, no create-task owed
- 260909 T058 claimed (swt)
- 260909 T057 done
- 260909 T057 claimed (frz)
- 260909 T056 done
- 260909 THEME r6 + MARKETING r4 implemented directly (promo primitives PromoStage/PromoText/spotlight, pointer floor on every control primitive, /about one-row header + hero login + plan cards, all FE gates green) — tasked 4→6 / 3→4, no create-task owed
- 260909 T056 claimed (ans)
- 260909 T055 done
- 260909 update-ssot THEME r6 (THEME-37✎ promotional exemption · 23✎ pointer floor · 29✎ sizes + Switch/promo roster · 8 15 18 19 27 34✎) · MARKETING r4 (5✎ 6✎ 11✎ 12✎ 13✎ 15✎); warning: T058 (todo) is based on THEME@5 — r6 touches neither `Switch`'s behaviour nor the builder, so its base only needs re-stamping at claim
- 260909 update-ssot THEME MARKETING start (promotional-surface exemption, pointer-based control sizes, /about header and plan cards)
- 260909 update-ssot AUTH r3 (AUTH-23✎ 24✎): the delta is already in the code (hotfix 3fbbe46), so tasked 2→3 and no create-task is owed
- 260909 update-ssot AUTH start
- 260909 T055 claimed (ask)
- 260909 create-task T055-T059 from TEMPLATE@4 POST@3 GEN@4
- 260909 THEME row reconciled to rev 5 (file r5, STATE read 4; the r5 delta was never registered)
- 260908 create-task TEMPLATE POST GEN start
- 260908 update-ssot TEMPLATE r4 (TEMPLATE-43+ 44+ 45+ 46+) · POST r3 (POST-62+) · GEN r4
