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
| GUIDE | 17 | 17 | - | 0 |
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
| structure-261008 | ready@261008 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |

## next
- Continue clear structural refactors after T660's verified main commit; review/structure-261008 retains the remaining decisions.
- T636–T646 standalone writing/origin tasks are complete on main; unrelated blocked qualifications remain separate.
- Editorial follow-up: doc-review ARCH; preserve independent review and open SSOT questions.

## log
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
- 261008 T654 start on main at7dd79345: centralize shared overlay scroll-lock ownership; structural review and unrelated local audit edits remain separate.
- 261008 review-code structure-261008 start: inspect readability, rule ownership, duplication and antipatterns; clear refactors authorized for direct sequential implementation.
- 261008 T646 done on main: real origin/champion lineage, first-call private purge and clean native publication handoff; integrated/race/browser/lint/build/codegen/spec checks pass
- 261008 T646 start on main ata9b25ea7: integrated origin/request lineage, lifecycle/privacy and both-theme browser/copy qualification; preserve independent review changes
- 261008 T646 freshness: MODEL36 missing-slot preparation and THEME30–32 navigation/dock/mobile changes retain referenced origin/request/export contracts; bases refreshed
- 261008 T645 done on main: source-bound current/retained/native inventory, zero-call instruction-only evaluation and 440 structural checks; owning/API-consumer/vet/build/format/spec checks pass
- 261008 T645 start on main atc01b5169: reproducible actual-composer evaluation and instruction-only language fixtures; preserve independent review changes
