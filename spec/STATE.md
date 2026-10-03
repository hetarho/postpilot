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
| POST | 30 | 30 | - | 0 |
| VOICE | 8 | 7 | VOICE-32✎ | 0 |
| GEN | 22 | 22 | - | 0 |
| MODEL | 28 | 28 | - | 0 |
| TMPL | 20 | 20 | - | 0 |
| GUIDE | 13 | 13 | - | 0 |
| EXPORT | 9 | 9 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 21 | 21 | - | 0 |
| MKT | 9 | 9 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 54 | 54 | - | 2 |
| CDS | 31 | 31 | - | 1 |
| BILL | 8 | 8 | - | 0 |
| MEM | 5 | 5 | - | 2 |
| QUAL | 7 | 7 | - | 0 |
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
| T524 | photo group block on the server | POST GEN QUAL TMPL | - | todo |
| T525 | writer places photo groups | GEN GUIDE TMPL | T524 | todo |
| T526 | reading view shows photo groups | POST GEN | T524 | todo |
| T527 | block editor edits photo groups | POST | T526 | todo |
| T528 | exports map photo groups | EXPORT | T526 | todo |
| T529 | template count reads as a suggested group | TMPL | - | todo |

## next
- implement-task T524 → T525 → T526 → T527 → T528 → T529 (photo groups; T529 has no dep).
- next: ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ remain pending for create-task ARCH.
- create-task VOICE r8 (VOICE-32✎ is already implemented by 68ae9a79: a verification-only task); update-ssot VOICE-31 remains open.

## log
- 261004 create-task GEN r22 POST r30 EXPORT r9 TMPL r20 GUIDE r13 QUAL r7 → T524 (server block), T525 (writer), T526 (reading view), T527 (editor), T528 (exports), T529 (template copy)
- 261004 update-ssot EXPORT r9: EXPORT-26✎ the site stylesheet carries the group rules for every post
- 261004 create-task GEN POST EXPORT TMPL GUIDE QUAL start: GEN-77+ GEN-78+ POST-105+ POST-106+ EXPORT-26+ TMPL-39✎ GUIDE-41✎ QUAL-10✎
- 261004 update-ssot GEN r22 POST r30 EXPORT r8 TMPL r20 GUIDE r13 QUAL r7: photo groups (콜라주 · 슬라이드, one caption) through writing, reading view, editor and the four exports; TMPL-39 decided
- 261004 update-ssot TMPL GEN EXPORT start: decide TMPL-39 — photo groups (collage/slide, one caption) through the post, rendering and export
- 261003 T523 done: captions excluded from intro/outro; local 31-second DB render and full test suites verified
- 261003 T523 start: keep generated and edited captions out of enabled regions
- 261003 create-task CLIP r54 → T523
- 261003 create-task CLIP start: CLIP-196+
- 261003 update-ssot CLIP r54: CLIP-196+ caption-free intro/outro intervals in preview and both exports
- 261003 update-ssot CLIP start: reserve enabled intro/outro spans for region text without captions
- 261002 T522 done: finish entry scroll and Naver body-with-tags copy; Node 24 FE 3054 tests and local CI checks passed
- 261002 T522 start: finish scroll and Naver body-with-tags copy
- 261002 create-task POST r29 EXPORT r7 → T522
- 261002 create-task POST EXPORT start
- 261002 update-ssot POST r29 EXPORT r7: scroll on finish entry and copy Naver body with tags
- 261002 update-ssot POST EXPORT start: finish navigation and Naver body-with-tags copy
- 261002 update-ssot VOICE r8: VOICE-32✎ the readiness meter says how many more sentences are needed (already live in 68ae9a79)
- 261002 update-ssot VOICE start: readiness meter states the sentences still needed
- 261002 T521 integration: rebased the template fix onto the voice rewrite hotfix; the sequential quiz continues with answer rewrites below 100% and preserves the photo
