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
| daily-credit-plans | open@260929 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 14 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 10 | 10 | - | 0 |
| QUOTA | 24 | 24 | - | 0 |
| POST | 25 | 25 | - | 0 |
| VOICE | 5 | 5 | - | 0 |
| GEN | 19 | 19 | - | 0 |
| MODEL | 20 | 20 | - | 0 |
| TMPL | 17 | 17 | - | 1 |
| GUIDE | 12 | 12 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 19 | 19 | - | 0 |
| MKT | 8 | 8 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 52 | 52 | - | 2 |
| CDS | 31 | 31 | - | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 5 | 5 | - | 2 |
| QUAL | 6 | 6 | - | 0 |
| GIFT | 2 | 2 | - | 0 |

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
| T464 | A browser render waits for its sampling job and draws from render-bound assets | CLIP ARCH | T463 | doing@260929.rr |
| T465 | Retire everything learned from finalized posts: learning, contrast rules, sentence feedback, rule comparison, profile validation and 규칙으로 저장 | VOICE GEN POST GUIDE QUOTA | - | todo |
| T466 | Retire the model lab's analyze comparison and the analyze A/B pair | MODEL VOICE GEN | T465 | todo |
| T467 | A post may have no voice (말투 없음) | POST GEN TMPL GUIDE ARCH VOICE LANG MODEL | T466 | todo |
| T468 | Voice directory: no automatic voice, no description seed, Korean voices, an optional 기본 | VOICE LANG POST GEN QUOTA | T467 | todo |
| T469 | 학습 글: pasted posts and prompt answers on the owner's photos, the readiness meter, and an explicit 말투 만들기 | VOICE POST QUOTA | T468 | todo |
| T470 | Count the fingerprint and compare a text against it | VOICE | T469 | todo |
| T471 | Analyse a voice as its fingerprint, show it in 말투 분석, undo one step, and project it in plain Korean | VOICE GEN GUIDE LANG POST | T470 | todo |
| T472 | ② shows the post's fingerprint beside its voice's | POST VOICE | T471 | todo |
| T473 | 검증: the AI writes one answered prompt in the voice, shown beside the owner's answer with the fingerprint comparison | VOICE QUOTA MODEL | T472 | todo |
| T474 | 말투 반영 비교: two write models write one answered prompt in a voice, judged beside the owner's answer | MODEL VOICE QUOTA | T473 | todo |
| T475 | On prod, keep only `맛집 리뷰 블로거 학습` with its pasted post | VOICE POST | T474 | todo |

## next
- next: implement-task T464 (the page waits for the browser render's sampling job and draws from render-bound assets, so its delivered clip carries the server-sampled scrim under CLIP-192)
- ideation daily-credit-plans continue (daily/bonus 15/290, 45/510, 85/1070, 235/3170 adopted; infrastructure break-even and capacity sensitivity checked, obtain real host specs/bill and representative media measurements; settle export quotas, workload limits, bonus expiry and model access); owner review of /IMPLEMENTATION-DECISIONS.md (the ambiguous calls of T414–T448 and T451–T456), then review-code the clip wave
- implement-task T465, then T466 … T475 in order (the voice as the owner's fingerprint; T475 is the one-time prod hand edit keeping `맛집 리뷰 블로거 학습`, run with the owner's yes); none of them runs beside another task that regenerates proto or adds a migration; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); the /about header overflow at 320px/200% text still wants a task (MKT THEME); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
- 260929 create-task VOICE r5 GEN r19 POST r25 GUIDE r12 QUOTA r24 MODEL r20 LANG r7 TMPL r17 ARCH r14 → T465–T475: retire finalized-post learning and the analyze comparison, 말투 없음, the directory, 학습 글 and readiness, fingerprint counting, the fingerprint analysis and projection, ② and 검증 comparisons, 말투 반영 비교, the prod hand edit; ARCH keeps tasked 9 for its CLIP-side r10–r13
- 260929 ideation daily-credit-plans open: full-use total infrastructure budgets KRW 913.475 / 2400.225 / 5001.225 / 15879.975 per payer; video reserve added back to avoid double counting actual worker bills; 36 budget and 6 scale cases reconciled in Decimal/Chromium; existing serial-worker and complex-fixture limits inspected, actual VPS specs/capacity remain unverified; interactive infrastructure calculator added; spec lint is blocked by concurrently drafted VOICE tasks T465–T472 missing from STATE, left to their planning session
- 260929 ideation daily-credit-plans resume: adopt daily/bonus 15/290, 45/510, 85/1070, 235/3170; examine infrastructure break-even, measured media capacity and whether worker scaling preserves contribution margin
- 260929 ideation daily-credit-plans open: KRW 1 credits, once-per-job rounding and 5-unit grants adopted; propose daily/bonus 15/290, 45/510, 85/1070, 235/3170 preserving 31-grant budgets; 72 scenario calculations include the 10% reserve, full illustrative server-render cost and 5.5% provider fee; interactive per-account/portfolio tables checked in Chromium, actual net profit remains dependent on measured costs and subscriber counts
- 260929 ideation daily-credit-plans open: owner's 21 screenshot costs total USD 0.10179; at illustrative KRW 1,400/USD, separate-row KRW 1 rounding adds 7.36%, KRW 10 adds 75.43%; recommend KRW 1 credits with one rounding per owner-visible job, awaiting adoption; grant examples rescaled without changing the assumed AI budgets
- 260929 ideation daily-credit-plans open: four KRW prices and the owner's (90% price - server allowance)/2 provider budget recorded; first proposal has progressive model access, server-export counts, 70/30 daily/bonus allocation and illustrative KRW 0.01 credits; provider minimum debit and real rendering cost remain unverified, video scope and bonus expiry await the owner
- 260929 T464 claimed (rr)
- 260929 T463 done: a `sample_browser_render` job runs one `sample` media stage on a worker and keeps the grounds on the browser render (migration 0105); `render_id` previews and caption frames draw on them; a dev-stack sampling of the T456 clip took 6 s
- 260929 /admin restored (adm): the running Vite server referenced missing optimized dependency files, returning 504 for @tanstack/react-virtual and failing the admin lazy import; restarting Vite rebuilt its cache without source changes; real-browser account/model/estimator/voucher tabs and reload pass with no browser errors, account API returns 200, AdminPage tests 5/5 pass
- 260929 create-task VOICE GEN POST GUIDE QUOTA MODEL LANG TMPL ARCH start (VOICE r5 GEN r19 POST r25 GUIDE r12 QUOTA r24 MODEL r20 LANG r7 TMPL r17 ARCH r14: the voice as the owner's fingerprint)
- 260929 T463 claimed (rr)
- 260929 T462 done: one originals-based ground sampler (the composition's frame arithmetic, each cut's own chain, xfade's dissolve and fade through black) serves the server render and `SampleGrounds`; a server render of the T456 clip is byte-identical to the one before; `clip.SampledGround` round-trips
- 260929 update-ssot VOICE r5 GEN r19 POST r25 GUIDE r12 QUOTA r24 MODEL r20 LANG r7 TMPL r17 ARCH r14: the voice is the owner's fingerprint — 학습 글 only, eight counted items plus a short AI part, a readiness meter, 검증 and 말투 반영 비교, optional voice and 기본, Korean only, learning, rules, versions and the analyze comparison removed; AUTH needs no change (VOICE-4); THEME-23 lost a stale example (wording); no active task affected — T462–T464 cite ARCH-45…51, not the changed ARCH-34
- 260929 update-ssot VOICE GEN POST GUIDE AUTH QUOTA MODEL LANG start (from ideation voice-tidy: the voice as the owner's fingerprint)
- 260929 ideation voice-tidy ready: 말투 is the owner's fingerprint — eight counted habits (endings, sentence-final marks, emoji, sentence/paragraph shape, opening/closing lines, adverbs, first person, headings/lists) plus a short AI description, the six axes dropped; 60 sentences from pasted posts or one shared prompt set on the owner's own photos; 검증, ② and the model lab's 말투 반영 비교 (replacing the analyze comparison) compare fingerprints with no extra call; Korean voices only, tied to no 분야 or template, several per owner by mood; no paid A/B now
- 260929 T456 follow-up: a rapid phrase in a sequence style is one raster of its style in the server render (CDS-4), so the draft preview now serves it as one (`sequenceDrawn`) and the frame endpoint refuses it; the first fix, which served it animated frames, is withdrawn; Chrome and server renders now match at 1.5 s (0.00 % of pixels off)
- 260929 T462 claimed (rr)
- 260929 create-task CLIP r52 → T462 T463 T464 (one originals-based ground sampler for both kinds; a media-worker sampling stage bound to the browser render; the page waits for it and sends render_id)
- 260929 ideation voice-tidy resume (open: axis set, readiness meter, prompt photos, VOICE-49, existing voices)
- 260929 create-task CLIP start (r52: CLIP-192+)
