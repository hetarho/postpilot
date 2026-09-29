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
| T470 | Count the fingerprint and compare a text against it | VOICE | T469 | todo |
| T471 | Analyse a voice as its fingerprint, show it in 말투 분석, undo one step, and project it in plain Korean | VOICE GEN GUIDE LANG POST | T470 | todo |
| T472 | ② shows the post's fingerprint beside its voice's | POST VOICE | T471 | todo |
| T473 | 검증: the AI writes one answered prompt in the voice, shown beside the owner's answer with the fingerprint comparison | VOICE QUOTA MODEL | T472 | todo |
| T474 | 말투 반영 비교: two write models write one answered prompt in a voice, judged beside the owner's answer | MODEL VOICE QUOTA | T473 | todo |
| T475 | On prod, keep only `맛집 리뷰 블로거 학습` with its pasted post | VOICE POST | T474 | todo |
| T476 | Define the KRW offer, Light tier and exact subscription windows | QUOTA BILL AUTH ARCH | - | todo |
| T477 | Grant daily credits and monthly bonuses once, with origin-period reservations | QUOTA AUTH ARCH | T476 | todo |
| T478 | Settle AI cost at a frozen FX rate and issue seven-day fault compensation | QUOTA CLIP ARCH | T477 | todo |
| T479 | Charge fixed KRW monthly/annual plans and prorate paid upgrades | BILL QUOTA ARCH | T477 | todo |
| T480 | Enforce curated free models and cumulative paid grades on every AI path | MODEL QUOTA CLIP ARCH | T474 T478 | todo |
| T481 | Show free groups, locked model grades and explicit eligible selections | MODEL QUOTA CLIP LANG ARCH | T480 | todo |
| T482 | Limit outputs to sixty seconds and reserve successful server exports | CLIP QUOTA ARCH | T477 | todo |
| T483 | Use monthly-bonus voucher presets and require paid redemption | GIFT QUOTA BILL ARCH | T479 | todo |
| T484 | Request and review payment refunds with confirmed entitlement reversal | BILL QUOTA ARCH | T479 T482 | todo |
| T485 | Show KRW billing, separate benefit clocks and transparent AI settlement | BILL QUOTA CLIP LANG ARCH | T478 T479 T482 T484 | todo |
| T486 | Publish the new plans on /plans and /about with eligible cost estimates | QUOTA MKT THEME LANG ARCH | T481 T485 | todo |
| T487 | Reset test entitlements and verify the complete pricing transition | QUOTA BILL MODEL CLIP AUTH MKT ARCH | T483 T486 | todo |

## next
- next: implement-task T476 (pricing foundation), then T477–T487 by their dependency graph; T480 consumes completed voice T474. Serialize all schema/proto writers with active T467–T474; no application or data reset has run. Pricing scope includes /plans and /about; infrastructure and PostgreSQL migration remain separate
- implement-task T470, then T471 … T475 in order (the voice as the owner's fingerprint; T475 is the one-time prod hand edit keeping `맛집 리뷰 블로거 학습`, run with the owner's yes); none of them runs beside another task that regenerates proto or adds a migration; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); T486 includes the /about header check at 320px/200% text; T479/T485 replace the current USD billing flow before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
- 260929 T469 done: 학습 글 are pasted posts and answers to 20 shared prompts (a photo prompt on the owner's own photo, private storage and a photo sweep), non-prose lines count nothing, a readiness meter needs 60 sentences and every part, and 말투 만들기 / 다시 분석 is an explicit AnalyzeVoice at 100% (migration 0110); the FE's second tab is 학습 글
- 260929 T468 done: no account gets a voice it did not make — no bootstrap or seed creates one, CreateVoice takes a name alone as a Korean voice not yet made, the description seed and voice languages are gone (migration 0109), the 기본 is optional and any voice deletes, and `/voices` is 내 글's row list with 기본 and 삭제 on the voice's own title row
- 260929 T468 claimed (e5): chain T468→T474, one commit per task; T475 (prod hand edit) waits for the owner
- 260929 T467 done: a post may have no voice — posts.voice_id is nullable and machine_baseline_voice_id is gone (migration 0108), a named voice must be active and made (VOICE_NOT_MADE), runs freeze the voice or its absence, a 말투 없음 prompt carries no voice bytes and ①'s picker offers 말투 없음, the made voices, unmade ones as 만드는 중 and 새 말투 만들기
- 260929 T467 claimed (vfp)
- 260929 pricing docs verified: 86 changed decisions mapped to T476–T487; dependency graph, current task bases and STATE rows consistent; spec lint has no errors (44 warnings, mainly consumed-chg/legacy result checks plus immutable T467 freshness); no application tests or data mutation needed for this documentation pass
- 260929 create-task pricing complete: T476–T487 consume QUOTA25 BILL5 MODEL21 CLIP53 GIFT3 AUTH11 MKT9; refreshed todo T468/T469 bases and T473/T474 shared-gate/selector integration; doing T467 unchanged (MODEL-31 unchanged); ARCH pending retained; implementation/reset not run
- 260929 create-task QUOTA BILL MODEL CLIP GIFT AUTH MKT start: pricing delta only; THEME/LANG reused unchanged and ARCH pending remains separate; refresh overlapping todo voice contracts without changing doing T467
- 260929 update-ssot daily-credit-plans complete: QUOTA r25 BILL r5 MODEL r21 CLIP r53 GIFT r3 AUTH r11 MKT r9; ideation converted, no application/data changes
- 260929 pricing SSOT impact: T467 is doing and keeps its immutable voice scope (MODEL-31 unchanged); T473 MODEL-16 and T474 MODEL-44 intersect the new model gate/order policy and need todo scope/base refresh at create-task; quota foundation will follow voice T474 to avoid concurrent schema/proto changes
- 260929 update-ssot QUOTA BILL MODEL CLIP GIFT AUTH MKT THEME LANG start: convert ready daily-credit-plans and then create implementation tasks; current T467 doing and T468–T475 remain owned by the voice session
- 260929 ideation daily-credit-plans ready: final free-model/zero-balance rules, removal of free daily 5 credits, provider-limit disclosure/no product daily count and paid-only new top-ups/vouchers adopted; all product questions resolved, offer/impact map reconciled; next update-ssot, then create-task; existing SSOT/application and other-session tasks unchanged
- 260929 T466 done: the model lab compares observe and write only — StartAnalyzeExperiment, the analyze apply and its confirmation, the analyze A/B pair and the experiment guard on voice deletion are gone (migration 0107), recommendation sets carry seven refs and the voice badges belong to write comparisons
- 260929 T466 claimed (vfp)
- 260929 ideation daily-credit-plans resume: owner replaces free-tier value access with separately managed free models only and a daily-usage-limit notice, exempts operators from model usage, adopts seven-day FX fallback and confirms no existing over-60-second outputs; reconcile free credits, limit ownership and paid top-up access before SSOT conversion
- 260929 ideation daily-credit-plans resume: owner requests continued questions; re-present the three unanswered model-exception, unavailable-FX and release-boundary choices without treating continuation as adoption; SSOT/application unchanged
- 260929 T465 done: nothing is learned from finalized posts any more — the learning and validation services, the three job kinds and the ten learning tables are gone (migration 0106), prompts carry the styleguide and pasted-sample excerpts only, StartRevision writes nothing to a voice, a reassignment keeps the machine baseline, ② 확정하기 finalizes at once and ③ holds 기억으로 저장 · 내보내기 · 발행 URL; natural_korean ends with the conditional voice sentence
- 260929 T465 claimed (vfp)
- 260929 ideation daily-credit-plans open: owner adopts annual remaining-year prorated upgrades, monthly→annual conversion at monthly renewal and non-expiring top-up packs 3000→1000/9000→3000/30000→10000 with no model/export unlock; owner then adopts previous-business-day FX rounded up to KRW 10/USD, job freeze and start-period reset attribution; supplier/owner cancellation pays confirmed usage only, service/unknown faults receive ceil-half compensation valid seven days; locked models preserve saved selections and show required plan, no separate server-count sales adopted; Free/Light/Basic/Pro/Max with Pro recommended adopted; monthly-bonus-sized 30-day voucher presets and seven-day unused full cash refunds/operator review adopted; final model/FX/scope questions pending; no SSOT/application changes
- 260929 ideation daily-credit-plans open: export top-ups round down; annual 19000/49000/99000/299000 adopted with daily/monthly benefits retained, 366-day cost sensitivity recorded; existing entitlements are test-only and may reset, accounts/content/operator access preserved; /plans and /about subscription updates required; annual upgrades, monthly→annual timing and top-up packs presented, awaiting answers; no SSOT/application/data reset performed
