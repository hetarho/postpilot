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
| T090 | Safe clip failure diagnostics | ARCH CLIP | T086 | blocked@260911 |

## next
- T090 implementation and local gates pass; confirm whether main push may also publish unrelated ancestor commits 6e81e3b/ca539e5, then verify remote CI/deployment before done; no paid retries. The 지침 screen (T087-T089) is done and committed on main
- spec maintenance owed: update-ssot VOICE for VOICE-42's frozen tag-count wording, update-ssot TMPL for the retired SLOT `label` vs `<ask label>` conflict, and update-ssot LANG to add the `clips` namespace
- T008 is the last PUB task and needs the owner present: re-read its base at PUB@4 · ARCH@2 first (it still says PUB@2 ARCH@1), then BEFORE `install` the owner must re-run `postpilot-agent setup` so the connection records driver signature smarteditor-one-20260910-a6, and the queued `20260905-test` job must be canceled or deliberately used as the smoke's own job. Read T042 T043 T045 T046 results for the live surveys and the wiring's typed preflight; update-ssot PUB for VIDEO-17 + TMPL-39 after T008 closes
## log
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
- 260910 T085 done; exact explicit server-ceiling approval, owned attempt-long previews and authoritative settlement UI; all local gates, race tests, 1469 frontend tests and ko/en 320/390/1024px browser checks pass, no paid call or push
- 260910 T085 claimed (crd); explicit server-priced approval, attempt-long local previews and authoritative settlement UI; preserve unrelated work and never replay paid starts
- 260910 T084 done; prepare-all verified inline media, exact frozen-policy reservation, bounded original rendering and cleanup; all local gates/races and nonroot 1GiB/2CPU smoke pass, no paid request or push
- 260910 T084 claimed (crd); prepare every bounded proxy before exact reservation and guarded inline AI, preserve original rendering and cleanup, user ceilings take priority
- 260910 T083 done; bounded inline/static transport, frozen multimodal quote/routing/usage policy and pre-enqueue URL gates; all local gates and targeted race tests pass, 1449 frontend tests, no paid call or push; paid runner remains closed until T084
- 260910 T083 claimed (crd); resume approved r9 transport and modality-price implementation, preserve the paid-runner guard until T084 and unrelated work
- 260910 create-task QUOTA done; T083–T086 now consume r9 with frozen multimodal quote/routing/settlement contracts, blocked T083 returned to todo by owner approval; archived T081–T082 unchanged
- 260910 create-task QUOTA start; reconcile r9 into T083–T086 with documented modality units and frozen quote/routing/accounting contracts; user approved resuming the blocked task, preserve archived T081–T082
