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
| POST | 29 | 29 | - | 0 |
| VOICE | 8 | 7 | VOICE-32✎ | 0 |
| GEN | 21 | 21 | - | 0 |
| MODEL | 28 | 28 | - | 0 |
| TMPL | 19 | 19 | - | 1 |
| GUIDE | 12 | 12 | - | 0 |
| EXPORT | 7 | 7 | - | 0 |
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

## next
- next: ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ remain pending for create-task ARCH.
- create-task VOICE r8 (VOICE-32✎ is already implemented by 68ae9a79: a verification-only task); update-ssot VOICE-31 remains open.
- Template request job content retention is open in JOB-RETENTION-TODO.md; doc-review VOICE for lint's split candidate.

## log
- 261002 T522 done: finish entry scroll and Naver body-with-tags copy; Node 24 FE 3054 tests and local CI checks passed
- 261002 T522 start: finish scroll and Naver body-with-tags copy
- 261002 create-task POST r29 EXPORT r7 → T522
- 261002 create-task POST EXPORT start
- 261002 update-ssot POST r29 EXPORT r7: scroll on finish entry and copy Naver body with tags
- 261002 update-ssot POST EXPORT start: finish navigation and Naver body-with-tags copy
- 261002 update-ssot VOICE r8: VOICE-32✎ the readiness meter says how many more sentences are needed (already live in 68ae9a79)
- 261002 update-ssot VOICE start: readiness meter states the sentences still needed
- 261002 T521 integration: rebased the template fix onto the voice rewrite hotfix; the sequential quiz continues with answer rewrites below 100% and preserves the photo
- 261002 T521 done: one learning screen and sequential quiz with live readiness; Node 24 FE 3048 tests, lint/FSD/style/build and local CI checks passed
- 261002 T520 done: write-stage check/reflection requests admitted by model registry; BE full tests and local CI checks passed
- 261002 T521 start
- 261002 T520 start
- 261002 create-task VOICE r7 → T520 (write-stage verification admission), T521 (learning quiz and navigation)
- 261002 create-task VOICE start
- 261002 update-ssot VOICE r7: learning screen before first analysis, sequential prompts with readiness progress, renamed tab, and analysis title removed
- 261002 update-ssot VOICE start: simplify voice learning flow, labels, and verification recovery
- 261001 T519 done: six production templates updated after the matching API rollout; SQLite backup and 49 posts/273 saved answers verified unchanged
- 261001 T519 start
- 261001 T518 done: required experience answers gate new writing; Node 24 FE 3,041 tests, BE full tests, build/lint/codegen passed
