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
| searchable-details | open@261007 |
| prompt-engineering | converted@261007 |
| creation-and-comparison-ux | open@261007 |
| storyline-first | converted@260927 |
| voice-tidy | converted@260929 |
| daily-credit-plans | converted@260929 |
| template-from-request | converted@261001 |
| familiar-video-editing-and-dubbing | converted@261004 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 20 | 20 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 38 | 38 | - | 0 |
| POST | 38 | 38 | - | 0 |
| VOICE | 16 | 16 | - | 0 |
| GEN | 27 | 27 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 25 | 25 | - | 0 |
| GUIDE | 20 | 20 | - | 0 |
| EXPORT | 11 | 11 | - | 0 |
| LANG | 9 | 9 | - | 0 |
| THEME | 33 | 33 | - | 1 |
| MKT | 10 | 10 | - | 0 |
| VIDEO | 7 | 7 | - | 0 |
| CLIP | 59 | 59 | - | 2 |
| CDS | 33 | 33 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 7 | 7 | - | 2 |
| QUAL | 8 | 8 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| EDIT | 5 | 5 | - | 0 |
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
| desktop-ux-policy-261007 | converted@261007 |
| all-261008 | converted@261008 |
| structure-261008 | converted@261008 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |
| T670 | Store the owner's photo order and reorder it | POST GEN | - | todo |
| T671 | Keep the pick order and reorder photos in ① | POST | T670 | todo |
| T672 | The writer reads the list as the owner's photo order | GEN GUIDE | - | todo |

## next
- implement-task T670, T672, then T671 (dep T670).
- Review the 17 unadopted choices in review/structure-261008; all ten clear refactor tasks T654–T663 are complete.
- T636–T646 standalone writing/origin tasks are complete on main; unrelated blocked qualifications remain separate.
- Editorial follow-up: doc-review ARCH; preserve independent review and open SSOT questions.

## log
- 261011 create-task POST r38 GEN r27 GUIDE r20: T670 (photo position + ReorderImages), T671 (pick-order confirm + ① strip reorder, dep T670), T672 (prompts read the owner's photo order).
- 261011 update-ssot POST r38 GEN r27 GUIDE r20 done: POST-116+ owner photo order (pick order, ① reorder) · POST-74✎ · GEN-7✎ GEN-77✎ GEN-82✎ the writer reads the list as the owner's photo order · GUIDE-41✎ arrange along it, a late photo joins its subject; no doing task in the blast radius; next create-task POST GEN GUIDE.
- 261011 update-ssot POST GEN GUIDE start: the owner's photo order — kept in pick order, reorderable in ①, given to the writer as usually the visit's order; a late photo of an earlier subject joins that subject.
- 261011 T669 done on main: a measured panel (Listbox, Toggletip) flips above only when its content does not fit below; code landed in c1774853, recorded and re-checked at 4dc93108.
- 261011 T669 start on main at 4dc93108: record the already-landed c1774853 against THEME r33.
- 261011 create-task THEME r33: T669 (measured panel flips only when its content does not fit below).
- 261011 create-task THEME start.
- 261011 update-ssot THEME r33 done: THEME-29✎ Listbox flips only when the room below cannot hold the list · THEME-30✎ shared placement rule for measured panels; no doing task in its blast radius; next create-task THEME.
- 261011 update-ssot THEME start: an anchored panel (Listbox, Toggletip) flips above only when the room below cannot hold its content and the room above is larger; code already landed in c1774853.
- 261009 T668 done on main: photo_groups 기본 지침 groups only under a caption naming one subject; a 와/및 caption joining different photos splits them.
- 261009 T668 start on main atca11432e: photo_groups 기본 지침 groups only under a one-subject caption.
- 261009 create-task GUIDE r19: T668 (photo_groups 기본 지침 → group only under a one-subject caption).
- 261009 update-ssot GUIDE r19 done: GUIDE-41✎ 사진은 한 장씩 groups only one composition with changing companions or one subject from several angles, under a one-subject caption; no doing task in its blast radius; next create-task GUIDE.
- 261009 update-ssot GUIDE start: today's posts still grouped two subjects under one 와/및 caption (입구와 매장, 리플릿과 테이블 세팅) at template 2장 묶음 places; tighten the grouping condition.
- 261009 T667 done on main: history rows drop the 내보내기 link and the export intent; 내보내기 가능 stays.
- 261009 T667 start on main ate982a61a: history rows drop the 내보내기 link and the export intent.
- 261009 T666 done on main: ① strip photos and clips open the shared large view (entities/post AttachmentViewer).
- 261009 T666 start on main at022ee33d: ① strip tiles open the shared large view.
- 261009 T665 done on main: ① and /posts/new show the photo slot above the 가제.
- 261009 out of scope: on a 390px phone the editor top row reads 글쓰기글 생성 — the location label and the step bar touch with no gap.
