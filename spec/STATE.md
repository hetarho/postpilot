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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 24 | 24 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 40 | 40 | - | 0 |
| POST | 48 | 48 | - | 0 |
| VOICE | 20 | 20 | - | 0 |
| GEN | 32 | 32 | - | 0 |
| MODEL | 37 | 37 | - | 0 |
| TMPL | 32 | 32 | - | 0 |
| GUIDE | 26 | 26 | - | 0 |
| EXPORT | 12 | 12 | - | 0 |
| LANG | 11 | 11 | - | 0 |
| THEME | 44 | 44 | - | 2 |
| MKT | 12 | 12 | - | 0 |
| VIDEO | 8 | 8 | - | 0 |
| CLIP | 78 | 78 | - | 2 |
| CDS | 38 | 38 | - | 1 |
| BILL | 10 | 10 | - | 0 |
| MEM | 8 | 8 | - | 2 |
| QUAL | 8 | 8 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 4 | 4 | - | 0 |
| EDIT | 6 | 6 | - | 0 |
| INFRA | 2 | 0 | all | 1 |

## review
| id | st |
|---|---|
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
| T803 | Unify route and step return controls | ARCH THEME | - | todo |
| T804 | Provide shared controls for actionable content | ARCH THEME | T803 | todo |
| T805 | Replace remaining text route returns | ARCH THEME | T804 | todo |
| T806 | Unify previous actions in authoring and comparison flows | ARCH THEME | T805 | todo |
| T807 | Align post and video template workspace headers | ARCH THEME | T806 | todo |
| T808 | Modernize both template directories and draft entries | ARCH THEME | T804 | todo |
| T809 | Align the remaining style resource directories | ARCH THEME | T808 | todo |
| T810 | Migrate actionable resource rows and cards | ARCH THEME | T809 | todo |
| T811 | Align video preparation with the writing guide | ARCH THEME | T804 | todo |
| T812 | Align video observation through completion | ARCH THEME | T811 | todo |
| T813 | Use shared media and radio controls in video editors | ARCH THEME | T812 | todo |
| T814 | Use shared actions throughout attachments and export | ARCH THEME | T804 | todo |
| T815 | Use shared fields in billing and refund forms | ARCH THEME | T814 | todo |
| T816 | Complete the route and control design-system audit | ARCH THEME | T807 T810 T813 T815 T821 T822 T823 T824 T825 | todo |
| T821 | Align template example and choice surfaces | ARCH | - | todo |
| T822 | Align remaining operational card and row surfaces | ARCH | - | todo |

| T823 | Align catalog and recommendation cards | ARCH THEME | - | todo |
| T824 | Align editing and observation surfaces | ARCH THEME | - | todo |
| T825 | Align comparison and plan disclosure surfaces | ARCH THEME | - | todo |

## work
| id | workers | total | done | remaining | running | review | blocked | updated | history |
|---|---|---|---|---|---|---|---|---|---|
| si8-261113 | 1 | 8 | 8 | 0 | 0 | 0 | 0 | 2026-11-12T20:35:28.177Z | [history](work/si8-261113.json) |

## next
- gusta-implement-task T803 T821 T822
- Video UX: owner approval is required to reset the vdux verification gate before final validation and landing.
