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
| POST | 39 | 39 | - | 0 |
| VOICE | 16 | 16 | - | 0 |
| GEN | 28 | 28 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 26 | 26 | - | 0 |
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
| T671 | Keep the pick order and reorder photos in ① | POST | T670 | doing@261011 |

## next
- implement-task T671 (in progress).
- Review the 17 unadopted choices in review/structure-261008; all ten clear refactor tasks T654–T663 are complete.
- T636–T646 standalone writing/origin tasks are complete on main; unrelated blocked qualifications remain separate.
- Editorial follow-up: doc-review ARCH; preserve independent review and open SSOT questions.

## log
- 261011 T670 done on main: images.position, confirm append, ReorderImages with POST_PHOTO_ORDER_STALE.
- 261011 create-task POST r39: folded into todo T671 (base POST@39).
- 261011 update-ssot POST r39 done: POST-116✎ 옮기기 picks a photo up and 여기로 / 맨 뒤로 put it down at every width; blast radius T671 (todo, updated).
- 261011 T673 done on main: the titles 기본 지침 sums up the core experience in implicative wording and binds the form's write roles.
- 261011 T673 start on main at 6e3bb7c5: titles 기본 지침 rewrite.
- 261011 create-task GEN r28 TMPL r26: T673 (titles 기본 지침 sums up in implicative wording).
- 261011 update-ssot GEN r28 TMPL r26 done: GEN-49✎ the title 기본 지침 sums up the core experience in implicative wording instead of listing facts, inventing no concrete fact · TMPL-52✎ it binds the form's write/answer_write roles; no doing task in its blast radius (T670 touches post storage only); next create-task GEN TMPL.
- 261011 update-ssot GEN TMPL start: titles became flat fact lists; the title 기본 지침 only forbade, and its `<write>` scope no longer matches the typed title roles.
- 261011 T672 done on main: the prompts read the list as the owner's photo order; order and photo_groups 기본 지침 arrange along it.
- 261011 T672 start on main at f3000bc6: the prompts read the list as the owner's photo order.
- 261011 T670 start on main at f3000bc6 (delegated worker in the main tree): images.position and ReorderImages.
- 261011 create-task POST r38 GEN r27 GUIDE r20: T670 (photo position + ReorderImages), T671 (pick-order confirm + ① strip reorder, dep T670), T672 (prompts read the owner's photo order).
- 261011 update-ssot POST r38 GEN r27 GUIDE r20 done: POST-116+ owner photo order (pick order, ① reorder) · POST-74✎ · GEN-7✎ GEN-77✎ GEN-82✎ the writer reads the list as the owner's photo order · GUIDE-41✎ arrange along it, a late photo joins its subject; no doing task in the blast radius; next create-task POST GEN GUIDE.
- 261011 update-ssot POST GEN GUIDE start: the owner's photo order — kept in pick order, reorderable in ①, given to the writer as usually the visit's order; a late photo of an earlier subject joins that subject.
- 261011 T669 done on main: a measured panel (Listbox, Toggletip) flips above only when its content does not fit below; code landed in c1774853, recorded and re-checked at 4dc93108.
- 261011 T669 start on main at 4dc93108: record the already-landed c1774853 against THEME r33.
- 261011 create-task THEME r33: T669 (measured panel flips only when its content does not fit below).
- 261011 create-task THEME start.
- 261011 update-ssot THEME r33 done: THEME-29✎ Listbox flips only when the room below cannot hold the list · THEME-30✎ shared placement rule for measured panels; no doing task in its blast radius; next create-task THEME.
- 261011 update-ssot THEME start: an anchored panel (Listbox, Toggletip) flips above only when the room below cannot hold its content and the room above is larger; code already landed in c1774853.
