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
| QUOTA | 25 | 25 | - | 0 |
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
| BILL | 5 | 5 | - | 0 |
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
| T486 | Publish the new plans on /plans and /about with eligible cost estimates | QUOTA MKT THEME LANG ARCH | T481 T485 | todo |
| T487 | Reset test entitlements and verify the complete pricing transition | QUOTA BILL MODEL CLIP AUTH MKT ARCH | T483 T486 | todo |

## next
- next: implement-task T486–T487 in order. Pricing scope includes /plans and /about; infrastructure and PostgreSQL migration remain separate
- update-ssot VOICE-31 (the 검증 job is named by ListVoiceChecks.active_job_id, the profile's by the analysis alone); the voice renewal T465–T475 is complete; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); T486 includes the /about header check at 320px/200% text; T479/T485 replace the current USD billing flow before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
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
- 260930 T480 claimed (p6): enforce curated free models and cumulative paid grades at catalog, selection and every AI admission boundary
- 260930 T479 done: fixed-KRW subscription, upgrade and paid-pack charges use persisted provider-reconciled intents, anchored renewal and one-time grants; ARCH-25/26/28 and deploy regression passed
- 260930 T479 claimed (p6): replace FX-dependent checkout with fixed KRW plans and packs, exact upgrade proration and reconciled subscription outcomes
- 260930 T478 done: official FX is frozen at paid admission, confirmed usage settles once, and service/unknown faults issue independent seven-day credit compensation; ARCH-26/28 and frontend/deploy regression passed
- 260930 T478 claimed (p6): freeze one official FX snapshot for AI admission and settle confirmed cost with fault compensation
- 260930 T477 done: paid daily/monthly credit and export windows are idempotent, upgrade and support transitions are atomic, origin-period holds survive resets; migration 0115 and full verification passed
- 260930 T477 claimed (p6): daily and monthly entitlements follow the completed T476 period and offer foundation
- 260930 T476 done: five KRW offers and Light identity are published; migration 0114 preserves accounts while widening the plan constraint; anchored KST month and 24-hour day periods, integer proration, zero free signup grants and Light dev fixture are verified
- 260930 T475 done: prod keeps one voice, `맛집 리뷰 블로거 학습` with its pasted post as its one 학습 글; the other 9 voices and their 학습 글 are gone, their 9 posts are 말투 없음 with text untouched (backup `postpilot-before-T475.db`)
