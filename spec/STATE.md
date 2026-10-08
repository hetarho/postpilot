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
- Continue clear structural refactors after T654's verified main commit; review/structure-261008 retains the remaining decisions.
- T636–T646 standalone writing/origin tasks are complete on main; unrelated blocked qualifications remain separate.
- Editorial follow-up: doc-review ARCH; preserve independent review and open SSOT questions.

## log
- 261008 T654 done on main: One body-scroll owner set restores the first baseline after the final overlay releases; nested/non-LIFO/StrictMode regressions and the full frontend suite pass.
- 261008 T654 start on main at7dd79345: centralize shared overlay scroll-lock ownership; structural review and unrelated local audit edits remain separate.
- 261008 review-code structure-261008 start: inspect readability, rule ownership, duplication and antipatterns; clear refactors authorized for direct sequential implementation.
- 261008 T646 done on main: real origin/champion lineage, first-call private purge and clean native publication handoff; integrated/race/browser/lint/build/codegen/spec checks pass
- 261008 T646 start on main ata9b25ea7: integrated origin/request lineage, lifecycle/privacy and both-theme browser/copy qualification; preserve independent review changes
- 261008 T646 freshness: MODEL36 missing-slot preparation and THEME30–32 navigation/dock/mobile changes retain referenced origin/request/export contracts; bases refreshed
- 261008 T645 done on main: source-bound current/retained/native inventory, zero-call instruction-only evaluation and 440 structural checks; owning/API-consumer/vet/build/format/spec checks pass
- 261008 T645 start on main atc01b5169: reproducible actual-composer evaluation and instruction-only language fixtures; preserve independent review changes
- 261008 T645 freshness: MODEL36 changes missing-slot setting preparation under MODEL92 only; evaluation/inventory/language contracts retained, base refreshed
- 261008 T644 done on main: maximum-tag copy preserves option/seed contracts; clean owner-wording exports/fallbacks, product-owned bands and shipped public owner-control claims pass affected checks
- 261008 T644 start on main ataa5e27b1: maximum-tag presentation, clean canonical exports and shipped owner-control copy; preserve independent review changes
- 261008 T644 freshness: THEME30–32 change navigation/docks/mobile density only, retaining THEME62 origin/export contracts; base refreshed
- 261008 T643 done on main: named owner-fenced technical post/authoring/test reads, honest current/prepared/captured views and blind denial; full affected FE coverage, server privacy and browser/lint/build/spec checks pass
- 261008 T643 start on main atf5fcc6c0: owner-scoped optional technical request views; preserve independent review changes
- 261008 T643 freshness: MODEL36/EDIT5 only change missing-slot integer1..16 preparation; THEME30–32 change navigation/docks/mobile density, retaining technical inspection contracts; bases refreshed
- 261008 T642 done on main: accessible aligned phrase origins, safe source details and editor/copy/caret continuity; full affected FE coverage, browser themes/reflow/contrast and lint/build/spec checks pass
- 261008 T642 start on main at8ce2d602: accessible current phrase origins and editor continuity; preserve independent review changes
- 261008 T642 freshness: THEME30–32 only change navigation/workspace/mobile density, retaining THEME62 origin review contracts; base refreshed
- 261008 T641 done on main: private test origin/request evidence, atomic purge-fenced champion publication and owner/kind/revision authoring inspections; owning/consumer/RPC/race/generator/build/spec checks pass
- 261008 T641 start on main at80f7fd8f: origin/request private test evidence and owner-scoped authoring inspection; preserve independent review changes
