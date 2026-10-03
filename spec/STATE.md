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
| POST | 31 | 30 | POST-105✎ POST-107+ | 0 |
| VOICE | 8 | 7 | VOICE-32✎ | 0 |
| GEN | 23 | 22 | GEN-77✎ GEN-78✎ GEN-14✎ GEN-40✎ GEN-79+ | 0 |
| MODEL | 28 | 28 | - | 0 |
| TMPL | 21 | 20 | TMPL-38✎ | 0 |
| GUIDE | 13 | 13 | - | 0 |
| EXPORT | 10 | 9 | EXPORT-15✎ | 0 |
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

## next
- create-task GEN POST TMPL EXPORT: groups of ≤3 of one orientation, always captioned; observed photo rotation (GEN r23 POST r31 TMPL r21 EXPORT r10).
- next: ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ remain pending for create-task ARCH.
- create-task VOICE r8 (VOICE-32✎ is already implemented by 68ae9a79: a verification-only task); update-ssot VOICE-31 remains open.

## log
- 261004 update-ssot GEN r23 POST r31 TMPL r21 EXPORT r10: groups of at most 3 of one orientation, every part captioned; photo rotation from the observation until the owner turns it
- 261004 update-ssot GEN POST TMPL EXPORT start: photo groups of at most 3, one orientation, always captioned; observe suggests photo rotation
- 261004 T529 done: template builder and 형식 안내 read count as a suggested group; photo-group wave complete
- 261004 T529 start
- 261004 T528 done: four exports and the Naver tab carry photo groups; FE suite green except the pre-existing ClipGeneration failure
- 261004 T528 start
- 261004 T527 done: block editor makes, edits and undoes photo groups; FE suite green except the pre-existing ClipGeneration failure
- 261004 T527 start
- 261004 T525 done: writer schemas, rules, normalization, template legend and the photo_groups 기본 지침; BE suite green
- 261004 T525 start
- 261004 out of scope: ClipGeneration.test.tsx 'reuses retained originals after done…' fails on a clean HEAD (no 바로 만들기 button), unrelated to photo groups
- 261004 T526 done: reading view renders photo groups (collage grid, slide strip); FE suite green except a ClipGeneration failure that also fails on a clean HEAD
- 261004 T526 start
- 261004 T524 done: GALLERY block validated, stored, served, finalized and measured on the server; BE suite and FE build pass
- 261004 T524 start
- 261004 create-task GEN r22 POST r30 EXPORT r9 TMPL r20 GUIDE r13 QUAL r7 → T524 (server block), T525 (writer), T526 (reading view), T527 (editor), T528 (exports), T529 (template copy)
- 261004 update-ssot EXPORT r9: EXPORT-26✎ the site stylesheet carries the group rules for every post
- 261004 create-task GEN POST EXPORT TMPL GUIDE QUAL start: GEN-77+ GEN-78+ POST-105+ POST-106+ EXPORT-26+ TMPL-39✎ GUIDE-41✎ QUAL-10✎
- 261004 update-ssot GEN r22 POST r30 EXPORT r8 TMPL r20 GUIDE r13 QUAL r7: photo groups (콜라주 · 슬라이드, one caption) through writing, reading view, editor and the four exports; TMPL-39 decided
- 261004 update-ssot TMPL GEN EXPORT start: decide TMPL-39 — photo groups (collage/slide, one caption) through the post, rendering and export
