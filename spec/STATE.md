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
| template-from-request | converted@261001 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 15 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ | 0 |
| AUTH | 11 | 11 | - | 0 |
| QUOTA | 32 | 32 | - | 0 |
| POST | 28 | 28 | - | 0 |
| VOICE | 6 | 6 | - | 0 |
| GEN | 21 | 21 | - | 0 |
| MODEL | 28 | 28 | - | 0 |
| TMPL | 19 | 19 | - | 1 |
| GUIDE | 12 | 12 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 21 | 21 | - | 0 |
| MKT | 9 | 9 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 53 | 53 | - | 2 |
| CDS | 31 | 31 | - | 1 |
| BILL | 8 | 8 | - | 0 |
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
| T519 | Refresh six production post templates with experience-first structure | TMPL POST GEN | T518 | doing@261001.exp |

## next
- next: finish T519 after the supporting code is live (six production templates); ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ remain pending for create-task ARCH.
- template request T507–T512 complete; job content retention is open in JOB-RETENTION-TODO.md.
- update-ssot VOICE-31 (the 검증 job is named by ListVoiceChecks.active_job_id, the profile's by the analysis alone); the voice renewal T465–T475 is complete; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26).

## log
- 261001 T519 start
- 261001 T518 done: required experience answers gate new writing; Node 24 FE 3,041 tests, BE full tests, build/lint/codegen passed
- 261001 T517 done: 문항 풀기 moves on to the next unanswered prompt with 건너뛰기, 글 붙여넣기 empties for the next post; full FE checks passed in a clean worktree
- 261001 T518 start
- 261001 create-task TMPL r19 POST r28 GEN r21 → T518 (required answer support), T519 (six production templates)
- 261001 create-task TMPL POST GEN start
- 261001 update-ssot TMPL r19 POST r28 GEN r21: required template answers gate new writing; optional answers still drop; revision keeps its existing edit path
- 261001 update-ssot TMPL POST GEN start: required experience fields for selected post templates
- 261001 create-task VOICE r6 → T517 (문항 풀기 opens the next unanswered prompt with 건너뛰기, 글 붙여넣기 empties for the next post; frontend only)
- 261001 create-task VOICE start
- 261001 update-ssot VOICE r6: VOICE-65+ a 학습 글 sheet keeps taking entries until the owner closes it (next unanswered prompt, 건너뛰기, blank paste form)
- 261001 update-ssot VOICE start: a 학습 글 sheet keeps taking entries until the owner closes it
- 261001 T516 done: master-only admin comparison cost, no public cost; full FE/BE and project checks passed
- 261001 T498 done: /plans cards carry tier, price, one button by billing state and four benefits; one below-card post/clip comparison with an inline clip disclosure; rendered checks at 320/390/1440 in both themes
- 261001 T515 done: master-only GetExchangeRate and /admin/costs (비용·환율) with every rate state; BE auth/cmd-api/plan and full FE checks passed
- 261001 T514 done: master sees billing, credit badge, popover and clip credit surfaces as a customer, blocked actions disabled; full FE checks passed
- 261001 T513 done: no rate or combo model labels on any customer response or screen, master included; full FE, BE 72 packages plus clip/store rerun alone (1595s), codegen idempotent
- 261001 create-task QUOTA r31–r32 BILL r8 MODEL r28 → T513 (no rate or model labels on customer responses), T514 (master sees customer screens), T515 (/admin 비용·환율 rate), T516 (comparison cost to /admin), re-cut T498 (one button on every card, four benefits, below-card comparison); T501 T502 T506 drop master cost
- 261001 update-ssot QUOTA r31–r32 BILL r8 MODEL r28: every /plans card one button, trimmed cards, saving once; QUOTA-68+ master sees customer screens as a customer (blocked actions disabled, no operator notice, rate or supplier cost; technical detail stays); rate and comparison cost on /admin's 비용·환율 tab
- 261001 T512 done: the template editor asks the 글 작성 모델 through the request box, and ③ opens a new template from the post; full FE checks passed
- 261001 T509 done: an owner cancels a queued or running template request; confirmed usage only is charged; migration 0126 widens the cancellation CHECKs; full FE/BE and project checks passed
- 261001 T506 done: normalized Elo leaderboard and no supplier-cost view verified
