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
| T501 | Complete comparisons with ranked candidates and result actions | MODEL GEN POST | T500 | todo |
| T502 | Replay ranked comparisons into a normalized Elo leaderboard | MODEL | T501 | todo |
| T503 | Add up to five candidates to the model lab and show every result | MODEL GEN QUOTA | T501 | todo |
| T504 | Rank every successful comparison candidate with optional ties | MODEL | T503 | todo |
| T505 | Apply or adopt an explicit candidate after ranking | MODEL GEN POST | T504 | todo |
| T506 | Show ranked-comparison Elo on the model leaderboard | MODEL | T502 T504 | todo |
| T508 | Run a template request on the 글 작성 모델 | TMPL QUOTA MODEL POST | T507 | todo |
| T509 | Cancel a running template request | TMPL QUOTA | T508 | todo |
| T510 | Quote one template request in credits | QUOTA TMPL | T508 | todo |
| T511 | Show a live placeholder preview beside the template composition | TMPL | - | todo |
| T512 | Ask the 글 작성 모델 from the template editor, and start a template from a post | TMPL POST QUOTA | T508 T509 T510 T511 | todo |

## next
- next: implement-task T501 for model comparison; T498 remains the earlier /plans task; ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ remain pending for create-task ARCH.
- implement-task T508 → T509/T510 → T512 for the template request; T511 (preview) has no dependency and can run beside them; job content retention is open in JOB-RETENTION-TODO.md.
- update-ssot VOICE-31 (the 검증 job is named by ListVoiceChecks.active_job_id, the profile's by the analysis alone); the voice renewal T465–T475 is complete; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); unmeasured: a rapid phrase in a hook-role style against the canvas width; a rapid phrase edge between two frames (e.g. 1020 ms at 30 fps) gives both phrases that frame, and the browser draws the earlier one there; a browser render started 23 s after a server render once showed no outcome for 30 minutes (not reproduced); bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26).

## log
- 261001 T507 done: the 형식 안내 is backend code served by GetFormatGuide and read by 원문 per locale; full FE/BE and project checks passed
- 261001 T507 claimed (tr): backend-owned 형식 안내 read by the client
- 261001 T500 done: five blind candidates, failed-only retry and stage-correct admission; FE/BE, migration, codegen and project checks passed
- 261001 create-task TMPL r18 QUOTA r30 POST r27 MODEL r27 → T507–T512 (backend 형식 안내, template request job, cancellation, estimate, live preview, request box and ③ entry); TMPL-6✎ amended in r18 to mirror TEMPLATE_MAX_PER_ACCOUNT
- 261001 create-task TMPL QUOTA POST MODEL start: template request, from a post, live preview, backend-owned 형식 안내
- 261001 update-ssot TMPL r18 QUOTA r30 POST r27 MODEL r27: template request (TMPL-58–TMPL-63), from a post (TMPL-64 POST-103), live preview (TMPL-65–TMPL-67), backend-owned 형식 안내 (TMPL-41✎); ideation template-from-request converted
- 261001 warn: MODEL r27 touches MODEL-9 MODEL-14 only, outside T499 (doing) and T500–T506
- 261001 update-ssot TMPL start: in-app template request, entry from a post, live preview, one shared 형식 안내 (ideation template-from-request)
- 261001 ideation template-from-request ready: in-app template request (input, fields, follow-up, correction, undo, cost, entry from a post) and a live placeholder preview
- 261001 T499 done: optional lab candidates saved atomically; full FE/BE and project checks passed
- 261001 T499 freshness: MODEL@26→27 changed template-request effort and stage use only; lab candidate contract unchanged
- 261001 T499 claimed (mc): optional model lab candidate persistence
- 261001 create-architecture ARCH r15: explicit editor-two and model-lab-two-to-five fan-out align I3 with MODEL-75
- 261001 create-architecture ARCH start: align explicit comparison fan-out invariant with two-to-five candidate model lab
- 261001 create-task MODEL r26 → refreshed T499–T506 for complete rankings with ties, result actions and normalized Elo
- 261001 create-task MODEL r26 start: refresh T499–T506 for complete rankings and Elo
- 261001 update-ssot MODEL r26: complete rankings with ties and normalized multi-candidate Elo; legacy outcomes remain in replay
- 261001 update-ssot MODEL start: replace independent three-step ratings with ranked multi-candidate comparisons and revised Elo
- 261001 create-task MODEL r25 GEN r20 POST r26 QUOTA r29 → T499–T506 (lab candidates, independent ratings, result actions, leaderboard); T498 QUOTA base refreshed
- 261001 create-task MODEL GEN POST QUOTA start: two-to-five candidate comparisons and per-candidate evaluations
