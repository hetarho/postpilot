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
| QUOTA | 5 | 5 | - | 0 |
| POST | 2 | 2 | - | 0 |
| VOICE | 1 | 1 | - | 1 |
| GEN | 2 | 2 | - | 0 |
| MODEL | 3 | 3 | - | 0 |
| TEMPLATE | 3 | 3 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUBLISH | 3 | 3 | - | 0 |
| LANG | 1 | 1 | - | 0 |
| THEME | 4 | 4 | - | 0 |
| MARKETING | 3 | 2 | MARKETING-6✎ 11✎ 16+ | 0 |
| VIDEO | 1 | 1 | - | 1 |
| BILLING | 2 | 0 | all | 0 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T007 | Agent automated test suite and LaunchAgent packaging | PUBLISH | T006 | doing@260907.ix |
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 T019 | todo |
| T019 | Naver editor mutations, the commit port and the daemon publisher wiring | PUBLISH | T018 T007 | blocked@260907 |
| T026 | The operator assigns a model to each estimator combo | QUOTA | T025 | todo |
| T027 | The estimate becomes a calculator on /plans | QUOTA THEME | T025 | todo |
| T028 | The plan ladder earns its animated stroke | THEME QUOTA | T027 | todo |

## next
- implement-task T026 → T027 → T028 (the estimator wave), then create-task for the AUTH·BILLING wave (self-signup, the anchor window, the payment-method bonus, the whole payment surface)
- PUBLISH chain: T007 needs one ARCH-27 run on the Mac (and CI green) to close · T019 blocked on the live editor survey → update-ssot PUBLISH → create-task re-decomposes it · T008 last, its base taking MARKETING@3 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes

## log
- 260907 T025 done — four operator-assigned combos price a post by photo·video·1000 chars in milli-credits, published through GetMyPlan; the worst-case reference post is gone
- 260907 T025 claimed (pw)
- 260907 create-task QUOTA THEME → T025..T028 (combos + published rates, the operator's assignment, the /plans calculator, the animated promotional stroke); T023's worst-case reference post and its copy are removed in T025/T027
- 260907 update-ssot QUOTA r5 THEME r4 done (the post estimate becomes proportional over adjustable characters·photos·videos across four operator-assigned combos, and a promotional surface may animate a gradient stroke on every option)
- 260907 WARN T023 shipped the worst-case 32-credit reference post and its caveat copy — QUOTA-36 r5 replaces both, so create-task must plan the removal, not just an addition
- 260907 T024 done — /about no longer claims a plan decides daily job counts or a model range, and its figures match the raised ladder; a claim-level assertion now guards the sentence
- 260907 T024 claimed (pw), base MARKETING@2→@3 QUOTA@3→@4 LANG@1 ARCH@1→@2: MARKETING r3 binds the access sentence and the CTA to self-signup SHIPPING, which it has not, so this task still writes the operator path
- 260907 T023 done — /plans compares four rungs side by side from md:, each stating about how many posts its grant buys, with pro marked under THEME-37 by the new stroke-accent role
- 260907 T023 claimed (pw), base QUOTA@3→@4 THEME@3 LANG@1 ARCH@1→@2: r4 changes the grant window and the bonus, neither of which the comparison table renders
- 260907 T022 done — the header carries the balance as a link to /plans, the popover reaches the ladder from every tier, and one 30s-stale GetMyPlan entry serves both
- 260907 T022 claimed (pw), base QUOTA@3→@4 THEME@3 ARCH@1→@2: r4 touches the grant window and the bonus, neither of which this header control reads
- 260907 T021 done — lots order by kind (monthly→bonus→purchased), a purchased kind exists for BILLING to fill, and an upgrade raises the running cycle on both tier-change paths. WARN sqlc slices emitted SQL by byte offset: a multi-byte character in a query comment silently generates unparseable SQL (pinned in usage.sql)
- 260907 T021 claimed (pw), base QUOTA@3→@4 ARCH@1→@2: r4 moves the grant WINDOW (QUOTA-37) while this task moves the lot ORDER and the upgrade top-up — the anchor is its own task in the AUTH bundle
- 260907 T020 done — the ladder is 220/575/1200, and GetMyPlan now publishes each rung's post estimate (32 credits per reference post) and the recommended rung; WARN spec lint flags every domain id over 6 chars (TEMPLATE PUBLISH MARKETING, and now BILLING) against FORMAT
- 260907 T020 claimed (pw), base QUOTA@3→@4 ARCH@1→@2: r4 (bonus, anchor window, term) and ARCH r2 (framing, billing context, auth mechanics) touch nothing this task implements
- 260907 update-ssot AUTH r2 QUOTA r4 ARCH r2 MARKETING r3 BILLING r2 done (self-signup with email as the login id and verification before the first session, Google sign-in, IP throttling + auto-releasing lockout, anchor-day grant window, payment-method bonus, the public CTA becoming the way in)
- 260907 WARN T020..T024 are todo against QUOTA@3 · MARKETING@2 · ARCH@1 — no behaviour they implement changed, but create-task must move their base; no doing task sits inside the five domains
- 260907 update-ssot AUTH QUOTA ARCH start (BILLING prerequisites)
- 260907 create-task QUOTA MARKETING THEME → T020..T024 (BE ladder+offers, purchased kind+order+top-up, header credit control, /plans reshape, /about copy); the money-dependent halves of QUOTA-34 and QUOTA-35 (par-rate checkout, the charge behind an upgrade) stay for BILLING's own tasks
- 260907 create-task QUOTA MARKETING THEME start
