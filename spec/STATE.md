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
| QUOTA | 27 | 27 | - | 0 |
| POST | 25 | 25 | - | 0 |
| VOICE | 5 | 5 | - | 0 |
| GEN | 19 | 19 | - | 0 |
| MODEL | 24 | 24 | - | 0 |
| TMPL | 17 | 17 | - | 1 |
| GUIDE | 12 | 12 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 20 | 20 | - | 0 |
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
| T494 | supplier cost and the credit conversion never reach a non-master | QUOTA MODEL | - | doing@260930.mk |
| T495 | per-post credit figures from recent usage on model and plan responses | QUOTA MODEL | T494 | todo |
| T496 | 글 1개당 크레딧 on model selectors, post creation and /plans | QUOTA MODEL | T495 | todo |

## next
- implement-task T494 (cost leak, prod-visible) then T495 → T496 (recent-usage per-post credits).
- next: no remaining tasks; create-task for pending ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+. Pricing scope includes /plans and /about; infrastructure and PostgreSQL migration remain separate.
- update-ssot VOICE-31 (the 검증 job is named by ListVoiceChecks.active_job_id, the profile's by the analysis alone); the voice renewal T465–T475 is complete; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26).

## log
- 260930 T494 claimed (mk): supplier cost and the credit conversion never reach a non-master
- 260930 create-task QUOTA r27 MODEL r24 → T494 (cost/conversion master-only + descriptor test), T495 (usage figures backend), T496 (screens)
- 260930 create-task QUOTA MODEL start: r27/r24 supplier cost, undisclosed conversion, recent-usage per-post credits
- 260930 T493 done: the 일괄 편집 current document is capped at max-h-field with a pinned copy button; full FE checks passed
- 260930 update-ssot QUOTA r27 MODEL r24 (+PRD F-9/§6.4): supplier cost master-only, credit↔KRW conversion undisclosed, recent-usage per-post credit estimates
- 260930 update-ssot QUOTA MODEL start: supplier cost master-only, credit conversion undisclosed, recent-usage per-post credits
- 260930 T493 claimed (rcs): capped 일괄 편집 current document with a copy control
- 260930 create-task THEME r20: T493
- 260930 update-ssot THEME r20: the 일괄 편집 current document is capped at max-h-field with a pinned copy control (fourth scroller exception)
- 260930 T492 done: 추천 조합 is 모델 관리's sixth tab (/admin back to four); the 일괄 편집 preview shows set changes and new causes; full FE checks passed
- 260930 T492 claimed (rcs): 추천 조합 as a 모델 관리 tab and the document's set diff
- 260930 T491 done: the models-v1 document carries `[recommendations]`, validated against its own registrations and applied in one transaction; unique set labels; full BE/FE checks passed
- 260930 T491 claimed (rcs): recommendation sets in the models-v1 document
- 260930 create-task MODEL r23: T491 document section + one-transaction apply, T492 sixth 모델 관리 tab + preview set diff
- 260930 create-task MODEL start: r23 recommendation sets in 모델 관리 and the document
- 260930 update-ssot MODEL r23: 추천 조합 becomes a sixth 모델 관리 tab (/admin back to four) and a `[recommendations]` section of the models-v1 document
- 260930 update-ssot MODEL start: recommendation sets inside 모델 관리 and in the models-v1 document
- 260930 T489 done: 추천 조합 admin tab adds, edits, moves and deletes sets with per-field refusals and read-time flags; /ai-models offers every set; full FE checks passed
- 260930 T489 claimed (rcs): 추천 조합 admin tab and every set on /ai-models
- 260930 T488 done: recommendation sets are provider rows with master-only save/delete/move under whole-set validation; providers.yaml keeps the provider only; full BE/FE checks passed
- 260930 T490 done: a master account keeps master (self SetUserPlan refused, billing never charges or moves it); full FE/BE checks passed on a clean worktree
- 260930 T490 claimed (mk): master keeps master, never charged
- 260930 create-task QUOTA r26 BILL r6 → T490 (T488/T489 were taken by a parallel create-task)
- 260930 update-ssot QUOTA r26 BILL r6: a master account leaves master only by another master or `api setplan`; it is never charged
