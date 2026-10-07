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

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |
| T641 | Extend test and authoring private evidence with origin-safe inspection | ARCH MODEL EDIT GEN POST MEM | T638 T640 T628 | todo |
| T642 | Render accessible phrase origins and preserve editor continuity | ARCH POST THEME EXPORT GEN | T637 | todo |
| T643 | Show safe current prepared and captured requests in named technical views | ARCH POST MODEL EDIT TMPL THEME | T640 T641 | todo |
| T644 | Present maximum tags and clean owner-controlled writing copy | ARCH POST TMPL GEN EXPORT MKT QUAL THEME | T634 T642 T643 | todo |
| T645 | Build reproducible prompt inspection and controlled writing evaluation | ARCH GEN MODEL LANG QUAL | T638 T639 T641 | todo |
| T646 | Qualify origin-aware writing inspection and clean publication handoff | ARCH GEN POST MODEL THEME EXPORT MKT LANG | T636 T637 T639 T641 T642 T643 T644 T645 T631 | todo |

## next
- T641 next for sequential implementation through T646 after the T640 commit.
- Editorial follow-up: doc-review ARCH; preserve independent review and blocked qualification.

## log
- 261008 T640 done on main: private actual post call captures, exact source/result/plan fences and provider-free configured previews; owning/consumer/RPC/race/generator/build/spec checks pass
- 261008 T640 start on main at3cbcccf0: persist safe actual post request witnesses and owner read-only previews; preserve independent review changes
- 261008 T640 freshness: MODEL36 only changes missing-slot setting preparation under MODEL92; post inspection policies unchanged, base refreshed
- 261008 T639 done on main: explicit video target/data roles, consumed frozen-story/speech schemas, declared stock applicability and actual native call metadata; owning/consumer/race/build/spec checks pass
- 261008 T639 start on main at52fe1ef5: admitted video/speech stage responsibilities and explicit material/target contracts; preserve independent review changes
- 261008 T639 freshness: MODEL36 only updates missing-slot setting preparation under MODEL92; video/speech inventory policies unchanged, base refreshed
- 261008 T638 done on main: kind/mode-scoped authoring fields and shared grammar, single style example sets and fenced approval-only memory proposals; owning/consumer/race/build/spec checks pass
- 261008 T638 freshness: exactT649 changes only missing-slot candidate admission/counts; retain integrated1..16 behavior and scoped composer contracts, bases refreshed
- 261008 T638 start on main at376709cd: selected authoring/style/memory composition and explicit material boundaries; preserve independent release/review changes
- 261008 T637 done on main: atomic current-result origins, conservative manual alignment and immutable attachment/plan fences; full product Go, impacted FE2870, current race, generators/build/spec pass
- 261008 T636 done on main: bounded semantic-origin calls, strict canonical-tail salvage, validated current sources and structural revision retention; full owning/consumer/API/race/build/spec checks pass
- 261008 T635 done on main: 50 actual-composer synthetic inventory modes, shared effective registry resolution and safe captured response/error/native evidence; full83 Go packages, final boundaries/races/API/build/spec pass
- 261008 T634 done on main: stage-specific meaning/evidence and block contracts, maximum tags with unchanged revision arrays preserved; full generation/guideline, post/test/API consumers and build/vet/spec pass
- 261008 T633 done on main: typed template roles and stock applicability frozen through ordinary/test paths, stage-owned source honesty and revision facts; full owning/consumer, API, race, build/vet and task-candidate spec checks pass
- 261008 T653 done: ff9 source pushed; full CI, actual deployment/health/CORS/R2 and three native media jobs pass; archive task and source-bound QA proof, preserve unrelated main work
- 261008 T653 health packaging repair: kernel proves remote512Mi OOM with Go1.26 SHA-linked32Mi entropy BSS under Rosetta; helper-only official v1.0.0-c2097c7c snapshot/checksum retains SHA/protocol and lowers measured RSS42,876→10,072KiB; no API/worker setting or budget change, final gates pending
- 261008 T653 release memory follow-up: live-owner authenticated Unix health probe and single-frame PNG decode remove unnecessary concurrent work; owning/race/deployment/bitwise-output checks pass, exact default256/512Mi release remains required
- 261008 T653 bounded-memory fix candidate: lightweight exact cgroup/disk observer, active owner-generation/runtime-bound worker health, and explicit probe filter thread limit; keep API256Mi/worker512Mi and all codecs/presets/CRF/decoder/default budgets, rerun affected final gates
- 261008 T653 final candidate refresh: include committed T631 at935b47e0 plus canonical-path CI repair; rerun invalidated frontend/backend/generated/image checks, preserve uncommitted T633/review work
- 261008 T653 start: repair removed Alpine zlib-r0 media build pins using official3.24 r1 packages, retain exact real-image gates and existing CI fixes; preserve active T631 changes
