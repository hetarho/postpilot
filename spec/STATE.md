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
| familiar-video-editing-and-dubbing | converted@261004 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 16 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ ARCH-11✎ ARCH-45✎ ARCH-60+ ARCH-61+ ARCH-62+ ARCH-63+ ARCH-64+ ARCH-65+ ARCH-66+ ARCH-67+ ARCH-68+ | 0 |
| AUTH | 11 | 11 | - | 0 |
| QUOTA | 35 | 35 | - | 0 |
| POST | 32 | 31 | POST-108+ | 0 |
| VOICE | 8 | 7 | VOICE-32✎ | 0 |
| GEN | 23 | 23 | - | 0 |
| MODEL | 31 | 31 | - | 0 |
| TMPL | 21 | 21 | - | 0 |
| GUIDE | 13 | 13 | - | 0 |
| EXPORT | 10 | 10 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 22 | 22 | - | 0 |
| MKT | 9 | 9 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 56 | 56 | - | 2 |
| CDS | 33 | 33 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 5 | 5 | - | 2 |
| QUAL | 7 | 7 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| INFRA | 2 | 0 | all | 1 |

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
| perf-cost-261004 | converted@261005 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T599 | Stream browser output and promote the verified private result | ARCH CLIP CDS | T592 T593 T595 T596 T597 T598 | todo |
| T600 | Use the browser composition engine throughout editing previews | ARCH CLIP CDS | T592 T593 T594 T595 T596 T597 T598 | todo |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | todo |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | todo |

## next
- finish T599/T600 verification, independent review and serial integration
- T603 technical source is retained with all real qualification gates blocked
- implement T604 after T599/T600 integration; preserve independent release gates

## log
- 261007 T603 source-only integration start: reviewed technical harness/AAC guard and actual pinned verifier proof; all six real qualification checks remain open and runtime blocked
- 261006 T597 integrated
- 261006 T601 integrated
- 261006 T596 integrated
- 261006 T598 integrated
- 261006 T602 integrated
- 261006 T595 integrated
- 261006 T590 integrated
- 261007 T590 resumed: isolated Colima CPU images and actual Docker generators recovered without interrupting Desktop/dev; independent audit reproduced pre-park accepted-output race for owned correction
- 261006 T593 integrated
- 261006 T594 integrated
- 261006 T592 integrated
- 261007 create-task native parity hints done: T593 source clock/preroll and T600 automatic geometry notes refined from existing native contracts; goals, acceptance, dependencies and SSOT remain unchanged
- 261007 create-task native parity hints start: unassigned T593/T600 require cumulative frame-aligned source audio and automatic caption geometry after edits; current worker contracts remain unchanged
- 261007 external checkout isolation: separate novice-UX work owns main changes and migration0145; analysis worker owns new migration0146, preserving both scopes without copying dirty main
- 261006 T591 integrated
- 261007 manage-work integration compatibility: temporary CLI preserves only46 inherited FORMAT/history warnings; structural errors and new warnings still reject; actual baseline/candidate conformance passed, installed package/skills/runtime JSON unchanged
- 261007 T591 independent native comparison correction: global pace/accent, declared disclosure only and exact-ms visibility require correction before integration; parent remains unchanged and prior approval invalidated
- 261007 create-task browser-media parallel hint refinement start: unassigned todo scope hints allow isolated overlapping worktrees with serial review/integration; task goals, acceptance, SSOT bases and dependencies stay unchanged
- 261006 parallel browser-media start: isolated task workers through T604 with dependency-aware dispatch and independent reviews; preserve T590 implementation and its Docker gate
