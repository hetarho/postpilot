# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ideation
| id | st |
|---|---|
| clip-source-observation-visibility | converted@260912 |
| clip-template-as-preset | converted@260917 |
| post-quality-and-related-links | converted@260923 |
| searchable-details | open@260926 |
| storyline-first | converted@260927 |
| voice-tidy | converted@260929 |
| daily-credit-plans | converted@260929 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 14 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 11 | 11 | - | 0 |
| QUOTA | 26 | 26 | - | 0 |
| POST | 25 | 25 | - | 0 |
| VOICE | 5 | 5 | - | 0 |
| GEN | 19 | 19 | - | 0 |
| MODEL | 21 | 21 | - | 0 |
| TMPL | 17 | 17 | - | 1 |
| GUIDE | 12 | 12 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 19 | 19 | - | 0 |
| MKT | 9 | 9 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 53 | 53 | - | 2 |
| CDS | 31 | 31 | - | 1 |
| BILL | 6 | 6 | - | 0 |
| MEM | 5 | 5 | - | 2 |
| QUAL | 6 | 6 | - | 0 |
| GIFT | 3 | 3 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |
| publishing-260922 | converted@260922 |
| published-quality-260924 | converted@260925 |
| clip-narrate-failure-260926 | converted@260926 |
| conformance-all-260927 | converted@260927 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|

## next
- next: no remaining tasks; create-task for pending ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+. Pricing scope includes /plans and /about; infrastructure and PostgreSQL migration remain separate.
- update-ssot VOICE-31 (the 검증 job is named by ListVoiceChecks.active_job_id, the profile's by the analysis alone); the voice renewal T465–T475 is complete; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26).

## log
- 260930 T490 done: a master account keeps master (self SetUserPlan refused, billing never charges or moves it); full FE/BE checks passed on a clean worktree
- 260930 T490 claimed (mk): master keeps master, never charged
- 260930 create-task QUOTA r26 BILL r6 → T490 (T488/T489 were taken by a parallel create-task)
- 260930 update-ssot QUOTA r26 BILL r6: a master account leaves master only by another master or `api setplan`; it is never charged
- 260930 ops: local and prod run without EXIM_API_KEY, so every paid model is unpriced and paid AI admission refuses (QUOTA-59) until a Korea Eximbank key is set
- 260930 T487 done: guarded one-time test entitlement reset, KRW-only wire cleanup and integrated pricing lifecycle verified locally
- 260930 T487 claimed (p6): prepare a guarded test-entitlement reset and verify the full pricing lifecycle
- 260930 T486 done: five KRW plans and eligible FX-priced estimates match public About; full FE/BE and local visual checks passed
- 260930 T486 claimed (p6): publish five KRW offers and eligible cost estimates on plans and About
- 260930 T485 done: KRW billing, distinct benefit clocks and confirmed-use AI settlement; full frontend and local gates passed
- 260930 T485 claimed (p6): show KRW billing, benefit clocks and AI settlement
- 260930 T484 done: reviewed owner refunds use payment-funded evidence, guarded provider cancellation and confirmed scoped entitlement reversal; full local verification passed
- 260930 T484 claimed (p6): owner refund requests, operator review and provider-confirmed scoped reversal
- 260930 T483 done: monthly-bonus voucher presets and transactional active-paid redemption; full backend/frontend/deploy verification passed
- 260930 T483 claimed (p6): update voucher presets and require authoritative paid coverage at redemption
- 260930 T482 done: sixty-second output bound and atomic origin-month server export reservations; current-binary CPU image smoke, full Go/frontend/deploy verification passed
- 260930 T482 claimed (p6): cap every clip output at sixty seconds and reserve one monthly server-export slot per successful stored result
- 260930 T481 done: free-group curation, shared plan-aware model pickers and atomic recommendation refusal are visible in ko/en; full local frontend/backend/deploy verification passed
- 260930 T481 claimed (p6): project free and paid model access into shared selectors, recommendation controls and operator curation
- 260930 T480 done: curated free and cumulative paid grades gate saved choices, clip quotes and every admitted AI call; frozen per-job rights keep existing work stable, free routes remain zero-price and pinned, and full backend/frontend checks passed
