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
| QUOTA | 30 | 30 | - | 0 |
| POST | 27 | 27 | - | 0 |
| VOICE | 5 | 5 | - | 0 |
| GEN | 20 | 20 | - | 0 |
| MODEL | 27 | 27 | - | 0 |
| TMPL | 18 | 18 | - | 1 |
| GUIDE | 12 | 12 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 21 | 21 | - | 0 |
| MKT | 9 | 9 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 53 | 53 | - | 2 |
| CDS | 31 | 31 | - | 1 |
| BILL | 7 | 7 | - | 0 |
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
| T498 | Simplify /plans cards and move post and clip estimates below them | QUOTA THEME | T497 | todo |
| T512 | Ask the 글 작성 모델 from the template editor, and start a template from a post | TMPL POST QUOTA | T508 T509 T510 T511 | todo |

## next
- next: implement-task T516 after T515 for comparison supplier cost; T498 remains the earlier /plans task; ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ remain pending for create-task ARCH.
- implement-task T512 for the template request box; job content retention is open in JOB-RETENTION-TODO.md.
- update-ssot VOICE-31 (the 검증 job is named by ListVoiceChecks.active_job_id, the profile's by the analysis alone); the voice renewal T465–T475 is complete; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26).

## log
- 261001 T509 done: an owner cancels a queued or running template request; confirmed usage only is charged; migration 0126 widens the cancellation CHECKs; full FE/BE and project checks passed
- 261001 T506 done: normalized Elo leaderboard and no supplier-cost view verified
- 261001 T505 done: explicit candidate apply and adoption flows verified
- 261001 T504 done: complete blind candidate ranks and equal ranks verified
- 261001 T503 done: saved C/D/E candidates and two-to-five-result lab flows verified
- 261001 T502 done: normalized multiway Elo, ties and scoped replay verified
- 261001 T501 done: ranked completion, candidate actions, retention and compatibility verified
- 261001 T511 done: a live placeholder preview stands beside the template composition at lg and behind a 구성 / 미리보기 switch below it
- 261001 T510 done: EstimateTemplateRequest states 무료 or about n credits for one request at catalog prices; full FE/BE and project checks passed
- 261001 T510 claimed (tr): one-call credit estimate for a template request
- 261001 T508 done: template requests run as account-owned jobs on the explicit write model, corrected up to three times, with the answer read through GetTemplateRequestResult; full FE/BE and project checks passed
- 261001 T508 claimed (tr): template request job on the 글 작성 모델
- 261001 T507 done: the 형식 안내 is backend code served by GetFormatGuide and read by 원문 per locale; full FE/BE and project checks passed
- 261001 T507 claimed (tr): backend-owned 형식 안내 read by the client
- 261001 T500 done: five blind candidates, failed-only retry and stage-correct admission; FE/BE, migration, codegen and project checks passed
- 261001 create-task TMPL r18 QUOTA r30 POST r27 MODEL r27 → T507–T512 (backend 형식 안내, template request job, cancellation, estimate, live preview, request box and ③ entry); TMPL-6✎ amended in r18 to mirror TEMPLATE_MAX_PER_ACCOUNT
- 261001 create-task TMPL QUOTA POST MODEL start: template request, from a post, live preview, backend-owned 형식 안내
- 261001 update-ssot TMPL r18 QUOTA r30 POST r27 MODEL r27: template request (TMPL-58–TMPL-63), from a post (TMPL-64 POST-103), live preview (TMPL-65–TMPL-67), backend-owned 형식 안내 (TMPL-41✎); ideation template-from-request converted
- 261001 warn: MODEL r27 touches MODEL-9 MODEL-14 only, outside T499 (doing) and T500–T506
- 261001 update-ssot TMPL start: in-app template request, entry from a post, live preview, one shared 형식 안내 (ideation template-from-request)
