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
| ARCH | 17 | 17 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 37 | 37 | - | 0 |
| POST | 34 | 34 | - | 0 |
| VOICE | 13 | 13 | - | 0 |
| GEN | 24 | 24 | - | 0 |
| MODEL | 33 | 33 | - | 0 |
| TMPL | 23 | 23 | - | 0 |
| GUIDE | 15 | 15 | - | 0 |
| EXPORT | 10 | 10 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 27 | 27 | - | 0 |
| MKT | 9 | 9 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 58 | 58 | - | 2 |
| CDS | 33 | 33 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 6 | 6 | - | 2 |
| QUAL | 7 | 7 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| EDIT | 2 | 2 | - | 0 |
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
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |

## next
- T603 remains blocked on real corpus/truth/current-provider/route/accounting/spend/human evidence; T604 remains blocked on original WASM provenance, physical devices and release/human/performance qualification.
- Technical main integration has passed local CI; verify the authorized main push's remote checks. Renderer, analysis, voice and distribution activation remain disabled.

## log
- 261007 main integration verified at033d7824: FE470/3714, whole Go and all19 local gates pass; forward149 and deployed-main features retained, original58 T590 draft files preserved separately
- 261007 T603/T604 blocked qualification persisted in task headers and STATE for remote delivery; technical contributions retained without completing real semantic/release acceptance
- 261007 main merge candidate ready: current origin/main authoring/XState/typography preserved; forward149 upgrades and legacy144 preservation pass, actual phone/desktop sticky preview checks pass; full CI and authorized main push pending
- 261007 T604 source-only delivery preserved: independently reviewed2b5d5915 merged normally; local HTTPS13, paired Mac16 and portable4 checks pass, six release acceptances remain open and runtime blocked
- 261007 browser-media-604 local technical work closed through T604: task commits and scoped receipts preserved; T603 real semantic qualification and T604 source/hardware/human/performance release gates remain blocked, no live deployment or activation
- 261007 T600 integrated
- 261007 T599 integrated
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
