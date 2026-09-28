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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 13 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 10 | 10 | - | 0 |
| QUOTA | 23 | 23 | - | 0 |
| POST | 24 | 24 | - | 0 |
| VOICE | 4 | 4 | - | 1 |
| GEN | 18 | 18 | - | 0 |
| MODEL | 19 | 19 | - | 0 |
| TMPL | 16 | 16 | - | 1 |
| GUIDE | 11 | 11 | - | 0 |
| EXPORT | 6 | 6 | - | 0 |
| LANG | 6 | 6 | - | 0 |
| THEME | 19 | 19 | - | 0 |
| MKT | 8 | 8 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 51 | 51 | - | 2 |
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
| T453 | Edit intro and outro slots in the storyline space | CLIP CDS ARCH | T452 | todo |
| T454 | Validate manual caption styles independently of the AI set | CLIP CDS ARCH | - | todo |
| T455 | Select and style individual captions in the draft preview | CLIP CDS ARCH | T454 | todo |
| T456 | Verify region and caption edits across preview and both renders | CLIP CDS ARCH | T453 T455 | todo |
| T460 | One closed-row guideline list with a 기본 지침 sheet | GUIDE | T459 | todo |

## next
- next: implement-task T453 (edit intro and outro slots in the storyline space), then T454–T456 in order, one commit per task; independently implement-task T460 (closed-row guideline list), one commit
- owner review of /IMPLEMENTATION-DECISIONS.md (the ambiguous calls of T414–T448 and T451), then review-code the clip wave
- ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); the /about header overflow at 320px/200% text still wants a task (MKT THEME); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
- 260929 T459 done: guidelines carry an optional title (≤40, migration 0104) offered on every create surface and never in a prompt
- 260929 T459 claimed (sm)
- 260929 T452 done: the storyline call and 바로 만들기's flow call draft the generated intro/outro slots in their one approved call; builds and revisions copy the slot words; region inputs join quotes, payloads and recovery
- 260929 T461 done: ① is 가제 → template fields → memo → photos; the 가제's Enter goes to the next field on screen
- 260929 T461 claimed (sm)
- 260929 create-task POST r24 → T461
- 260929 update-ssot POST r24: ①'s memo moves below the template data fields; no active task/worker affected
- 260929 T458 done: a memory row is its text over one badge line with 수정/삭제 icons; one form saves text, kind and tags
- 260929 T458 claimed (sm)
- 260929 T457 done: a storyline tile opens its attachment large in a wide sheet, walking paragraphs then 빠진 사진
- 260928 T452 claimed (p15)
- 260928 T457 claimed (sm); owner asked to run T457–T460 in order, one commit per task
- 260928 create-task POST r23 GUIDE r11 MEM r5 → T457–T460; T460 waits on T459
- 260928 create-task POST GUIDE MEM start
- 260928 update-ssot POST r23 GUIDE r11 MEM r5: storyline tiles open large; memory rows compact with one edit form; guidelines one closed-row list with optional titles and a 기본 지침 sheet (추가 / 적용 안함); no active task/worker affected
- 260928 update-ssot POST MEM GUIDE start (enlarge storyline photos; compact memory cards with one edit mode; guideline title list, preset picker modal)
- 260928 T451 done: region slots project into the plan on slot/preset/correction/generation writes; owner-fixed overflow refuses by slot; ambiguous calls in IMPLEMENTATION-DECISIONS.md
- 260928 T451 claimed (p15); owner asked to run T451–T456 in order, one commit per task
- 260928 T450 done: project region persistence/API and compatibility verified with all local gates; commit/push requested, stop here and leave T451–T456 unstarted
- 260928 T450 verification: ff900701 fixes recovery writes rejected after wall-clock rollback; the deterministic regression and repeated recovery tests pass
