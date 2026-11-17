# STATE
> spec control tower. Every agent: ① read this file before working ② record the session start through `vibe` BEFORE questioning/reasoning/implementing ③ reflect product/task state here immediately; session coordination uses `vibe` on the coordination branch.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ideation
| id | st |
|---|---|
| searchable-details | open@261007 |
| creation-and-comparison-ux | open@261007 |
| friendly-model-names | open@261013 |
| post-faithful-clip | ready@261111 |
| instagram-photo-post | open@261111 |
| related-post-links | open@261111 |
| unmeasured-search-exposure | open@261111 |
| job-content-retention | open@261111 |
| voice-fidelity | open@261116 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 26 | 26 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 40 | 40 | - | 0 |
| POST | 54 | 54 | - | 0 |
| VOICE | 22 | 22 | - | 0 |
| GEN | 34 | 34 | - | 0 |
| MODEL | 39 | 39 | - | 0 |
| TMPL | 35 | 35 | - | 0 |
| GUIDE | 28 | 28 | - | 0 |
| EXPORT | 12 | 12 | - | 0 |
| LANG | 11 | 11 | - | 0 |
| THEME | 47 | 47 | - | 2 |
| MKT | 13 | 13 | - | 0 |
| VIDEO | 9 | 9 | - | 0 |
| CLIP | 81 | 81 | - | 2 |
| CDS | 38 | 38 | - | 1 |
| BILL | 10 | 10 | - | 0 |
| MEM | 8 | 8 | - | 2 |
| QUAL | 8 | 8 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 5 | 5 | - | 0 |
| EDIT | 6 | 6 | - | 0 |
| INFRA | 2 | 0 | all | 1 |

## review
| id | st |
|---|---|
| audio-true-peak-261117 | converted@261117 |
| tidy-261116 | converted@261116 |
| design-system-panels-261114 | converted@261114 |
| video-editing-ux-261114 | converted@261114 |
| performance-design-system-261113 | converted@261113 |
| video-preparation-guide-261112 | converted@261112 |
| video-request-retention-261112 | converted@261112 |
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
| desktop-ux-policy-261007 | converted@261007 |
| all-261008 | converted@261008 |
| structure-261008 | converted@261008 |
| perf-quality-261109 | converted@261109 |
| perf-quality-si4-261109 | converted@261109 |
| perf-quality-sil5-261110 | converted@261110 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 T748 T750 | blocked@261111 |
| T697 | Expose Content-Range on private storage and gate the deploy on it | ARCH INFRA | - | blocked@261016 |
| T878 | Start the learning chat from the owner's own experience | VOICE THEME | T877 | todo |
| T879 | Name the experience wherever a thread answer is listed or edited | VOICE | T877 | todo |
| T887 | Reuse each true-peak FIR sample across its four phases | ARCH | - | blocked@261117 |
| T890 | Add the photo observation test wire contract | MODEL ARCH | - | todo |
| T891 | Store test-owned photos and the test output kind | MODEL VIDEO ARCH | T890 | todo |
| T892 | Upload, confirm and sweep test-owned photos | MODEL POST ARCH | T891 | todo |
| T893 | Admit and project photo observation tests | MODEL GEN ARCH | T892 | todo |
| T894 | Run photo observation tests that stop at each contestant's observation | MODEL VIDEO GEN ARCH | T893 | todo |
| T895 | Map photo observation tests and test photo uploads in the client | MODEL ARCH | T890 | todo |
| T896 | Build the photo observation test setup at /tests/photos | MODEL POST THEME ARCH | T889 T895 | todo |
| T897 | Review, reveal and adopt photo observation test results | MODEL ARCH | T894 T896 | todo |

## work
| id | workers | total | done | remaining | running | review | blocked | updated | history |
|---|---|---|---|---|---|---|---|---|---|
| si8-261113 | 1 | 8 | 8 | 0 | 0 | 0 | 0 | 2026-11-12T20:35:28.177Z | [history](work/si8-261113.json) |
| tidy-261116 | 3 | 14 | 14 | 0 | 0 | 0 | 0 | 2026-11-16T08:07:39.788Z | [history](work/tidy-261116.json) |

## next
- gusta-implement-task T879 T890
