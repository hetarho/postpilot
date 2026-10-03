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
| POST | 31 | 31 | - | 0 |
| VOICE | 8 | 7 | VOICE-32✎ | 0 |
| GEN | 23 | 23 | - | 0 |
| MODEL | 28 | 28 | - | 0 |
| TMPL | 21 | 21 | - | 0 |
| GUIDE | 13 | 13 | - | 0 |
| EXPORT | 10 | 10 | - | 0 |
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
| T532 | show and copy photos turned, and let the owner turn them | POST EXPORT | T531 | todo |

## next
- implement-task T532, then push (owner asked for commit and push).
- next: ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ ARCH-34✎ remain pending for create-task ARCH.
- create-task VOICE r8 (VOICE-32✎ is already implemented by 68ae9a79: a verification-only task); update-ssot VOICE-31 remains open.

## log
- 261004 T531 done: observed and owner photo rotation stored and served; BE and FE suites green
- 261004 T531 start
- 261004 T530 done: groups of at most three, one orientation, always captioned; BE and FE suites green
- 261004 T530 start
- 261004 create-task GEN r23 POST r31 TMPL r21 EXPORT r10 → T530 (groups of three), T531 (rotation on the server), T532 (rotation on screen)
- 261004 create-task GEN POST TMPL EXPORT start: GEN-77✎ GEN-78✎ GEN-79+ POST-107+ TMPL-38✎ EXPORT-15✎
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
