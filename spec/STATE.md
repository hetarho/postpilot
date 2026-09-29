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
| T481 | Show free groups, locked model grades and explicit eligible selections | MODEL QUOTA CLIP LANG ARCH | T480 | todo |
| T482 | Limit outputs to sixty seconds and reserve successful server exports | CLIP QUOTA ARCH | T477 | todo |
| T483 | Use monthly-bonus voucher presets and require paid redemption | GIFT QUOTA BILL ARCH | T479 | todo |
| T484 | Request and review payment refunds with confirmed entitlement reversal | BILL QUOTA ARCH | T479 T482 | todo |
| T485 | Show KRW billing, separate benefit clocks and transparent AI settlement | BILL QUOTA CLIP LANG ARCH | T478 T479 T482 T484 | todo |
| T486 | Publish the new plans on /plans and /about with eligible cost estimates | QUOTA MKT THEME LANG ARCH | T481 T485 | todo |
| T487 | Reset test entitlements and verify the complete pricing transition | QUOTA BILL MODEL CLIP AUTH MKT ARCH | T483 T486 | todo |

## next
- next: implement-task T481–T487 in order. Pricing scope includes /plans and /about; infrastructure and PostgreSQL migration remain separate
- update-ssot VOICE-31 (the 검증 job is named by ListVoiceChecks.active_job_id, the profile's by the analysis alone); the voice renewal T465–T475 is complete; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); T486 includes the /about header check at 320px/200% text; T479/T485 replace the current USD billing flow before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
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
- 260930 T475 claimed (e5): owner said yes to the prod edit; T468–T474 pushed (c1fc3c11)
- 260929 T474 done: 말투 반영 비교 is a lab write comparison sourced from one voice (migration 0113) — both write models write 검증's piece for one answered prompt from one frozen snapshot, the blind review shows each piece's fingerprint comparison beside the owner's answer, the verdict counts on the write board with adoption as its only follow-up, the job names no voice so deletion is never blocked, and the model lab's compare and history pages gain the 말투 반영 tab
- 260929 T474 claimed (e5)
- 260929 T473 done: 검증 is the voice's third tab — one `check_voice` job (migration 0112: `voice_checks`, one per voice) on the write selection writes one answered prompt with its answer withheld, through the shared admission as one write call; results stay listed newest first with 내 답 · AI 글, the fingerprint comparison, 이전 분석으로 검증 and a retry; an unanswered prompt is answered first, a photo prompt needs `vision`
- 260929 T473 claimed (e5)
- 260929 T472 done: ② shows 말투 지문 under the measurements — `GetPostFingerprint` counts the post's blocks against its voice's current analysis per request with no call (not applicable for 말투 없음, no content, or a deleted or unmade voice), through a `PostContents` port over `post.CurrentContent`; the shared `FingerprintComparison` widget reads each item's headline facet and every facet on demand
- 260929 T472 claimed (e5)
- 260929 T471 done: a voice's analysis is its counted fingerprint plus one analyze call's AI part in `voice_analyses` (current and previous; migration 0111 drops the versioned profile, converts nothing and clears every 기본), 이전 분석으로 되돌리기 undoes one step, the notice says 새 학습 글 N편 or 학습 글이 바뀌었어요, prompts carry `[말투]` in plain Korean or portable habits for English, `ending_run` joins the 기본 지침, and the voice's first tab is 말투 분석
- 260929 T471 claimed (e5)
- 260929 T470 done: `internal/voice` counts the eight-item fingerprint over 학습 글, a text or a post's blocks (thresholds, examples, facets) and compares a text against a voice; `SegmentSentences` keeps `!!` and a trailing emoji with their sentence
- 260929 T470 claimed (e5)
