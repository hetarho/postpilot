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
| ARCH | 2 | 1 | ARCH-1✎ 5✎ 23✎ | 0 |
| AUTH | 2 | 1 | AUTH-1✎ 2✎ 3✎ 4✎ 5✎ 17✎ 30✎ 33+ 34+ 35+ 36+ 37+ 38+ 39+ 40+ 41+ | 0 |
| QUOTA | 4 | 3 | QUOTA-9✎ 12✎ 37+ 38+ | 0 |
| POST | 2 | 2 | - | 0 |
| VOICE | 1 | 1 | - | 1 |
| GEN | 2 | 2 | - | 0 |
| MODEL | 3 | 3 | - | 0 |
| TEMPLATE | 3 | 3 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUBLISH | 3 | 3 | - | 0 |
| LANG | 1 | 1 | - | 0 |
| THEME | 3 | 3 | - | 0 |
| MARKETING | 3 | 2 | MARKETING-6✎ 11✎ 16+ | 0 |
| VIDEO | 1 | 1 | - | 1 |
| BILLING | 2 | 0 | all | 0 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T007 | Agent automated test suite and LaunchAgent packaging | PUBLISH | T006 | doing@260907.ix |
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 T019 | todo |
| T019 | Naver editor mutations, the commit port and the daemon publisher wiring | PUBLISH | T018 T007 | blocked@260907 |
| T021 | The purchased lot kind, consumption by kind, and the upgrade top-up | QUOTA | T020 | todo |
| T022 | The header credit control that leads to the ladder | QUOTA THEME | - | todo |
| T023 | The plan comparison reshaped, with a recommended rung and post estimates | QUOTA THEME | T020 | todo |
| T024 | The public plans copy tells the truth about what a plan decides | MARKETING QUOTA | T020 | todo |

## next
- create-task AUTH QUOTA ARCH MARKETING BILLING next — every prerequisite is decided, so the whole payment surface is decomposable; T020..T024 (todo) must take QUOTA@4 · MARKETING@3 · ARCH@2 in that pass
- implement-task T021 → T022 → T023 → T024 (pw, in that order); every base must move to QUOTA@4 · MARKETING@3 · ARCH@2 on claim
- PUBLISH chain: T007 needs one ARCH-27 run on the Mac (and CI green) to close · T019 blocked on the live editor survey → update-ssot PUBLISH → create-task re-decomposes it · T008 last, its base taking MARKETING@3 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes

## log
- 260907 T020 done — the ladder is 220/575/1200, and GetMyPlan now publishes each rung's post estimate (32 credits per reference post) and the recommended rung; WARN spec lint flags every domain id over 6 chars (TEMPLATE PUBLISH MARKETING, and now BILLING) against FORMAT
- 260907 T020 claimed (pw), base QUOTA@3→@4 ARCH@1→@2: r4 (bonus, anchor window, term) and ARCH r2 (framing, billing context, auth mechanics) touch nothing this task implements
- 260907 update-ssot AUTH r2 QUOTA r4 ARCH r2 MARKETING r3 BILLING r2 done (self-signup with email as the login id and verification before the first session, Google sign-in, IP throttling + auto-releasing lockout, anchor-day grant window, payment-method bonus, the public CTA becoming the way in)
- 260907 WARN T020..T024 are todo against QUOTA@3 · MARKETING@2 · ARCH@1 — no behaviour they implement changed, but create-task must move their base; no doing task sits inside the five domains
- 260907 update-ssot AUTH QUOTA ARCH start (BILLING prerequisites)
- 260907 create-task QUOTA MARKETING THEME → T020..T024 (BE ladder+offers, purchased kind+order+top-up, header credit control, /plans reshape, /about copy); the money-dependent halves of QUOTA-34 and QUOTA-35 (par-rate checkout, the charge behind an upgrade) stay for BILLING's own tasks
- 260907 create-task QUOTA MARKETING THEME start
- 260907 create-ssot BILLING r1 done (17 decisions, 0 [?]): USD prices charged in KRW at the previous business day's rate, subscription-day anchor, monthly + annual (10 months for 12), immediate upgrade and scheduled downgrade, cancel as a scheduled stop, no retry on a failed charge, 7-day refund on untouched purchases, a 100-credit bonus for registering a card
- 260907 WARN BILLING assumes self-signup, an account email and the anchor move — AUTH, QUOTA and ARCH all carry decisions it contradicts until they are reopened
- 260907 create-ssot BILLING start
- 260907 update-ssot QUOTA r3 MARKETING r2 THEME r3 done (grants +10/15/20 %, par-rate credit purchase, kind-ordered consumption, header credit entry to /plans, /plans reshaped with a recommended rung, one promotional border exception; the money side moved out to BILLING, which does not exist yet)
- 260907 WARN T008 (todo) quotes MARKETING@1 and must take @2; no doing task sits inside QUOTA, MARKETING or THEME
- 260907 T007 tests+packaging complete and green on linux; only the macOS ARCH-27 / CI half is unverified. WARN launchd.Uninstall used to bootout unconditionally, so `go test` on a Mac would have stopped a real agent — now behind a test seam
- 260907 update-ssot QUOTA MARKETING start (pricing rework: grant bonus, always-on credit purchase, lot order, /plans entry + comparison, plans copy drift)
- 260907 T007 claimed (ix), base PUBLISH@2→@3 per create-task's r3 ruling that its tests+packaging scope is unaffected
- 260907 T019 blocked: Prepare's editor model contradicts the live SmartEditor in 3 places (no image placeholder, nothing opens the settings layer so tags/category/visibility never resolve, readback needs frame-scoped observation PUBLISH-20 does not enumerate); daemon wiring deliberately left unwired
- 260907 T019 claimed (ix)
- 260907 T019 (mu) and T007 (cx) reclaimed to todo on the owner's explicit instruction — both sessions ended with their work committed and the tree clean at 111bf53
- 260907 WARN T019: SmartEditor has no image placeholder (the photo button IS the file chooser), so Prepare's placeholder-then-upload phase cannot be observed — images must be inserted at their position during uploading_photos, keeping PUBLISH-13 stages monotonic
- 260907 T019 claimed (mu)
