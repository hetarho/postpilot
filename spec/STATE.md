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
| post-quality-and-related-links | open@260918 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 5 | 5 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 14 | 14 | - | 0 |
| POST | 8 | 8 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 7 | 7 | - | 0 |
| MODEL | 11 | 11 | - | 0 |
| TMPL | 7 | 7 | - | 1 |
| GUIDE | 3 | 3 | - | 0 |
| EXPORT | 4 | 4 | - | 0 |
| PUB | 5 | 5 | - | 0 |
| LANG | 3 | 3 | - | 0 |
| THEME | 14 | 14 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 2 | 2 | - | 1 |
| CLIP | 40 | 40 | - | 1 |
| CDS | 23 | 23 | - | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 1 | 1 | - | 2 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | doing@260910.e2e |
| T177 | Release QA viewing checklist | CDS CLIP | T176 | blocked@260916 |
| T279 | every hand-kept enum mirror is pinned to the generated enum | ARCH | T008 | todo |
| T282 | the agent maps proto at one adapter and keeps preflight out of main | ARCH | T008 | todo |
| T283 | SmartEditor scripts are files with a DOM test; naver is plan vs driver | ARCH | T282 | todo |
| T284 | the agent generates only the protos it uses and drops the survey harness | ARCH | T008 T281 | todo |

## next
- nothing is unblocked: every remaining task waits on T008 (another session) or is blocked (T177)
- five group directories (말투, 글 템플릿, 지침, 기억, 영상 템플릿) still draw their own heading under the phone's group row, which THEME-38 says yields there — only /posts and /clips follow it
- the clip `release-smoke` stage is still red at HEAD on this host (9 of 28 modes end in `no result`) and still needs a fix task
- a `review-code` pass over the new memory domain is worth a look before it grows
## log
- 260921 T296 done; the current destination is told by the accent everywhere, the group rail is denser than the primary one, the phone group row's name is its own menu trigger, and a list's dock is the floating control alone (nav)
- 260921 T296 created from THEME r14 and claimed (nav)
- 260921 THEME r14: the second level is told by density too, the phone group row's name IS its menu trigger, the current destination takes the accent, and a list's dock loses its plane (nav)
- 260921 update-ssot THEME start (nav)
- 260921 T295 done; estimator cards and plan estimates now share value/balanced/premium/top, enforce same-level purpose registrations, and safely migrate only matching legacy pairs (eql)
- 260921 T295 claimed (eql)
- 260921 T295 created from MODEL r11 QUOTA r14: shared level vocabulary, ordinal migration, server enforcement and purpose-specific admin filtering (ops-model-levels)
- 260921 create-task MODEL QUOTA start (ops-model-levels)
- 260921 MODEL r11 QUOTA r14: estimator combos now share the four model levels and accept only same-level per-purpose registrations; ordinal migration drops mismatched assignments (ops-model-levels)
- 260921 update-ssot MODEL QUOTA start (ops-model-levels)
- 260921 TMPL r7..r7 no-op (no code impact); the same stale marker prose in ExportPanel, BlockList, README and PRD was aligned in passing — comments and reference docs only, ARCH-25 green
- 260921 create-task TMPL start (alt)
- 260921 TMPL r7: TMPL-17 23 39 cite the current Naver photo marker, and a slot placeholder is told from it by its brackets (alt)
- 260921 update-ssot TMPL start (alt)
- 260921 T294 done; the Naver photo marker carries its caption folded to one `_`-joined token (`사진_1_비_뒤의_바다_사진`), captionless blocks keep the bare form, and both Naver snapshots moved by exactly those lines
- 260921 T294 created from EXPORT r4: one task, the converter's fold plus its tests, goldens and the panel's guidance line (alt)
- 260921 create-task EXPORT start (alt)
- 260921 EXPORT r4: the Naver photo marker carries its caption folded into one double-clickable token; EXPORT-24's caption copy stays and its reason follows (alt)
- 260921 update-ssot EXPORT start (alt)
- 260920 T293 done; ① carries the 기억 사용 checkbox (autosaved, flag only) and ③ carries 기억으로 저장 with the candidate sheet that creates only what is checked — the MEM chain is complete
