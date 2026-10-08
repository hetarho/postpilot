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
| POST | 36 | 36 | - | 0 |
| VOICE | 16 | 16 | - | 0 |
| GEN | 26 | 26 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 25 | 25 | - | 0 |
| GUIDE | 18 | 18 | - | 0 |
| EXPORT | 11 | 11 | - | 0 |
| LANG | 9 | 9 | - | 0 |
| THEME | 32 | 32 | - | 1 |
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
| T664 | Photos stand alone by default | GUIDE GEN | - | todo |

## next
- implement-task T664.
- Review the 17 unadopted choices in review/structure-261008; all ten clear refactor tasks T654–T663 are complete.
- T636–T646 standalone writing/origin tasks are complete on main; unrelated blocked qualifications remain separate.
- Editorial follow-up: doc-review ARCH; preserve independent review and open SSOT questions.

## log
- 261008 create-task GUIDE r18: T664 (photo_groups 기본 지침 → single photos by default).
- 261008 update-ssot GUIDE r18 done: GUIDE-41✎ photo_groups 기본 지침 → 사진은 한 장씩; no doing task in its blast radius; next create-task GUIDE.
- 261008 update-ssot GUIDE start: photo_groups 기본 지침 flips to single photos by default; a group only where one caption fully describes every photo.
- 261008 review-code structure-261008 complete: 10 clear refactors T654–T663 implemented, verified and committed sequentially; 17 structural choices retained, with semantic duplication kept separate.
- 261008 T663 done on main: Published pricing/ranking/source-grade comments now describe current entitlement ownership; Go scanner comparison proves executable tokens are unchanged.
- 261008 T663 start on main atce30ae11: Make model/plan API comments teach the current entitlement and pricing ownership without changing executable policy.
- 261008 T662 done on main: Nine contexts share an explicitly named 128-bit lowercase-hex entity ID primitive; local test/error seams and distinct bearer-token/fingerprint protocols remain intact.
- 261008 T662 start on main at220bd5d6: Give identical 128-bit lowercase-hex entity IDs one named platform primitive while keeping domain and credential lifecycles distinct.
- 261008 T661 done on main: StartRevision builds complete typed frozen options and encodes once; legacy adapters and known-answer wire bytes retain protocol/cap/language/profile semantics.
- 261008 T661 start on main at4dd37b2c: Make one typed revision-options builder own the complete admitted payload and serialize it once.
- 261008 T660 done on main: Removed seven zero-production-consumer editor/selector files and obsolete exports; current direct/shared editing and active/optional/pair/lab model flows remain covered.
- 261008 T660 start on main ate483c67a: Remove verified zero-production-consumer editor code while retaining the current authoring and model configuration flows.
- 261008 T659 done on main: Template builder recovery uses an owning typed metadata reader; null/arrays/primitives/malformed JSON no longer break direct editing or change canonical source on mount.
- 261008 T659 start on main at4bce3175: Keep template metadata recovery behind a typed owning reader without allowing non-object JSON to break direct editing.
- 261008 T658 done on main: A dedicated TypeScript-AST build module rewrites only model-owned named imports, retaining aliases/type declarations and lazy UI; build regressions cover the actual route.
- 261008 T658 start on main atc74ba432: Make the route build boundary explicit and handle inline type/value aliases without eagerly importing page UI.
- 261008 T657 done on main: Refund reads, refetches and named actions expose explicit plain domain models; money/status/nested-presence behavior and operation-specific invalidation remain intact.
- 261008 T657 start on main at2e81e638: Keep refund transport messages behind the subscription adapter and expose plain domain results to its features.
- 261008 T656 done on main: Raw billing SQL now names its reader and writer explicitly; ordinary reads avoid writer contention and both transaction constructors retain read-your-writes and rollback.
- 261008 T656 start on main atf08ff71f: Give generated and hand-written billing SQL the same explicit reader/writer and transaction ownership.
- 261008 T655 done on main: Catalog writes share one dependent-view invalidation helper; refresh retains its returned tab, other views refetch and saved owner choices remain untouched.
- 261008 T655 start on main at26b87a32: Give every catalog curation action one explicit dependent-query invalidation behavior.
- 261008 T654 done on main: One body-scroll owner set restores the first baseline after the final overlay releases; nested/non-LIFO/StrictMode regressions and the full frontend suite pass.
