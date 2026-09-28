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
| voice-tidy | open@260929 |

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
| T456 | Verify region and caption edits across preview and both renders | CLIP CDS ARCH | T453 T455 | todo |

## next
- next: implement-task T456 (verify region and caption edits across preview and both renders)
- owner review of /IMPLEMENTATION-DECISIONS.md (the ambiguous calls of T414–T448 and T451–T455), then review-code the clip wave
- ideation voice-tidy continues (open: the axis set under the owner's Korean-research rule, readiness-meter numbers, the prompt photo source, the VOICE-49 analyze experiment, existing voices), then update-ssot VOICE GEN POST GUIDE AUTH QUOTA; ideation searchable-details continues: pooled 유입 검색어 screenshots teach the product's own write prompt, credits paid monthly after verification; open: consent, tying a keyword to the post it reached; Later: update-ssot CLIP-163 then create-task ARCH CLIP (real-GPU validation, profile approval, concurrency); the /about header overflow at 320px/200% text still wants a task (MKT THEME); BILL carries the Toss placeholder and USD pricing before any card rollout; unmeasured: a rapid phrase in a hook-role style against the canvas width; bare legacy acceptance labels (`A5:`, `A2/A3:`) with no job number remain in ~124 backend and ~110 frontend test comments (T411/T412 results); doc-review split candidates left as blocks, since other SSOTs cite their parts (THEME-24 THEME-38 MODEL-37 GUIDE-26)

## log
- 260929 T455 done: ②'s caption sheet picks any approved style from renderer-drawn tiles, keeps the owner size (a size the drawn style cannot take holds the save on that field), and follows undo/redo
- 260929 T455 claimed (p15)
- 260929 T454 done: an owner may give a caption any approved style outside the AI set; one rule draws every caption, so a set change restyles only captions naming no style and moves the revision only then; a size the drawn style cannot take is refused as caption_size with its range
- 260929 T454 claimed (p15)
- 260929 T453 done: ② edits the intro/outro slots around the storyline body before any template, body or plan, and ① offers 사용 안 함; a drawn slot's words are the plan's rows, and the settings, slot and correction saves share one write lane
- 260929 T453 claimed (p15)
- 260929 ideation voice-tidy open: 말투 learns only from prose the owner wrote (pasted posts, per-분야 photo/situation prompts, a readiness meter, one analysis at 100%); read-only 말투 분석 with research-defined axes; 검증 beside the owner's answer; optional voice and 기본; drops 대조 규칙, finished-post learning, 문장 의견, the 버전 기록 tab, the seed and 규칙으로 저장
- 260929 ideation voice-tidy start (does the voice apply; what 말투/프로필/버전/측정·분석/여섯 성향/대조 규칙/검증 mean; list and detail pages)
- 260929 T460 done: one closed-row guideline list (기본 지침 in use, then the owner's) with a 기본 지침 sheet and a one-form edit
- 260929 T460 claimed (sm)
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
