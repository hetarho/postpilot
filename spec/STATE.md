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
| VOICE | 2 | 2 | - | 1 |
| GEN | 5 | 5 | - | 0 |
| MODEL | 8 | 8 | - | 0 |
| TMPL | 4 | 4 | - | 1 |
| GUIDE | 2 | 2 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 4 | 4 | - | 0 |
| LANG | 2 | 2 | - | 0 |
| THEME | 9 | 9 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 3 | 3 | - | 0 |
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
- Finish the authorized T091 push and verify its exact commit in CI and the deployed backend; no further paid model call or user-media replay
- T008 is the last remaining task and needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job
- spec maintenance owed: update-ssot VOICE for VOICE-42's frozen tag-count wording, update-ssot TMPL for the retired SLOT `label` vs `<ask label>` conflict, update-ssot LANG to add the `clips` namespace, and update-ssot PUB for VIDEO-17 + TMPL-39 once T008 closes
## log
- 260911 T091 release started (fix); owner requested commit/push, isolate clip changes from concurrent work and verify remote CI/deployment without paid calls
- 260911 T091 done (fix); corrected constrained output schemas, actual observation/planning and 15s Korean-captioned MP4 passed; all local gates/races green, reported test cost USD 0.003055 and conservative total USD 0.056321 within approval; no commit/push/deploy
- 260911 T091 resumed (fix); owner approved synthetic live verification up to USD 0.10 total, no historical replay or production settings change
- 260911 T091 blocked (fix); inspected the actual wire and official contracts without an evidenced cause; ask for bounded synthetic live verification, no paid call or speculative code change
- 260911 T091 claimed (fix); prioritize the actual analyze 400 and a playable end-to-end result over additional defensive features; preserve concurrent work and existing credit ceilings, no historical replay
- 260911 T090 done; the push-scope hold cleared, 61fe2d2 is on main and CI/deploy are green on 7327d93
- 260911 update-ssot VOICE TMPL LANG done (mnt); VOICE-42 drops the tag count and GEN-46✎ excludes rule comparison, TMPL scopes the retired `label` to `slot`, LANG-7 gains `billing` `clips`; all four revs are wording-only so tasked=rev
- 260911 update-ssot VOICE TMPL LANG start (mnt); the three spec-maintenance items owed in next
- 260911 T089 done; the 후보 queue is a counted disclosure below the list with sequential bulk accept/dismiss and per-row refusals; all local gates and day/night browser checks pass
- 260911 T089 claimed (cnd)
- 260911 T088 done; the 지침 page is the list with one docked 새 지침 sheet; all local gates and day/night browser checks pass
- 260911 T090 local verification passed (diag); diagnostics, privacy/usage/race tests, production media image and unchanged-side gates pass; push awaits scope approval for two unrelated ancestor commits, remote CI/deployment pending
- 260911 T088 claimed (gdl)
- 260911 T087 done; two-level navigation chrome, plane-separated and stuck to the viewport, with one composed chrome-offset token; all local gates and CDP browser checks pass
- 260911 T090 claimed (diag); approved metadata-only clip diagnostics follow-up, no policy/billing/retry changes; verify and push only this fix
- 260911 clip diagnostics start (diag); scope the approved metadata-only logging fix, preserve concurrent navigation work; no paid retry or provider payload logging
- 260911 T087 claimed (nav)
- 260910 T081–T086 delivered through 820eabc as six verified task commits; CI https://github.com/hetarho/postpilot/actions/runs/34486036362 and backend rollout https://github.com/hetarho/postpilot/actions/runs/34486036393 pass, Workers deployment succeeds; no live model completion or historical rebilling
- 260910 T086 done; 25 authenticated release cases, 20-source/30-minute stress, 1GiB/2CPU real-media tests and all local gates pass; corrected AAC timing, no paid completion or production mutation, series ready for authorized push/CI
- 260910 T086 claimed (crd); authenticated local release regressions, counted fake provider and bounded real-media stress; no paid provider calls or production mutations, push only after all gates pass
