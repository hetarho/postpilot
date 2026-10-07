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
| T628 | Run private binary tournaments with exact metering and explicit winner publication | ARCH MODEL QUOTA GEN LANG | T622 T624 T625 T626 T627 T630 | todo |
| T631 | Complete UX wiring and qualify creation settings and sixteen-entry tests | ARCH THEME POST CLIP EDIT VOICE MODEL QUOTA | T622 T623 T624 T625 T626 T627 T628 T629 T630 | todo |
| T633 | Preserve template material roles and declared stock-rule stages | ARCH TMPL GUIDE GEN | T632 T631 | todo |
| T634 | Align post stage contracts and grounded maximum-tag behavior | ARCH GEN GUIDE POST LANG | T633 | todo |
| T635 | Expose real prompt composition and effective request snapshots | ARCH MODEL GEN | T632 | todo |
| T636 | Generate reviewable origins through observation planning and revision | ARCH GEN VOICE MEM MODEL POST | T634 T635 | todo |
| T637 | Persist current-result origins and preserve them through manual edits | ARCH GEN POST MEM MODEL | T636 | todo |
| T638 | Scope setting style and memory composers to their consumed outputs | ARCH EDIT VOICE MEM GUIDE TMPL GEN MODEL | T633 T635 T626 T630 | todo |
| T639 | Align existing video and speech prompt-stage responsibilities | ARCH GEN MODEL GUIDE LANG CLIP | T633 T635 | todo |
| T640 | Capture and inspect the owner post effective product requests | ARCH POST MODEL GEN | T635 T637 | todo |
| T641 | Extend test and authoring private evidence with origin-safe inspection | ARCH MODEL EDIT GEN POST MEM | T638 T640 T628 | todo |
| T642 | Render accessible phrase origins and preserve editor continuity | ARCH POST THEME EXPORT GEN | T637 | todo |
| T643 | Show safe current prepared and captured requests in named technical views | ARCH POST MODEL EDIT TMPL THEME | T640 T641 | todo |
| T644 | Present maximum tags and clean owner-controlled writing copy | ARCH POST TMPL GEN EXPORT MKT QUAL THEME | T634 T642 T643 | todo |
| T645 | Build reproducible prompt inspection and controlled writing evaluation | ARCH GEN MODEL LANG QUAL | T638 T639 T641 | todo |
| T646 | Qualify origin-aware writing inspection and clean publication handoff | ARCH GEN POST MODEL THEME EXPORT MKT LANG | T636 T637 T639 T641 T642 T643 T644 T645 T631 | todo |

## next
- Deliver verified mobile T650–T652 on main, preserving unrelated concurrent work and current committed domain changes.
- Editorial follow-up: doc-review ARCH; pre-push gates remain separate from this local presentation verification.

## log
- 261008 T650 T651 T652 done: combined522/4133 FE and64-entry/57-route/580 browser checks plus lint/build pass; manually integrated because package integration treats inherited FORMAT/lint warnings as fatal
- 261008 mobile integration refreshes committed main T649 at051bb0e5; exact-slot preparation remains intact, current THEME32/ARCH20 mobile contracts unchanged, reviews rebind to the updated parent
- 261007 create-task THEME r32 consumed into T650–T652; explicit parallel authorization overrides sequential execution for this isolated mobile group
- 261007 update-ssot THEME r32: compact focused setup and phone header location plus app-wide height-aware composition; T648 dock48px contract and active T649/T628 behavior preserved
- 261007 update-ssot THEME start: mobile height and focused first-use location contracts, parallel work explicitly authorized for this request
- 261007 T648 done on main: compact translucent docks, bounded composers and48px actions across the app; consumer608 plus final55, build/lint and Chromium196 routes/308 geometry/70 continuity checks pass
- 261007 T627 done on main: immutable single-factor factories and private complete outputs, exact checkpoint/retry plans and current direct/refined references; owning/domain/wiring tests, builds/vet and FE124 consumers pass
- 261007 Docker Desktop termination complete: 9 app processes received SIGTERM; 4 remaining or respawned processes required SIGKILL; independent scans confirmed no Docker.app processes or port 7678 listener; separate Colima/Lima and persistent data files retained
- 261007 T630 done on main: named durable setting editing, private reference/metadata continuity, terminal paginated summaries and atomic domain receipts; full FE4077, owning Go/wiring, builds/lint/codegen and real viewport checks pass
- 261007 T630 final freshness: committed T647 supplies THEME30 routing delta; named setting policies50/53/60 remain unchanged, T630 base refreshed and current route consumers verified
- 261007 local API diagnostic: frontend2564 responds but API7678 health resets; Docker Desktop engine _ping times out without bytes and compose startup/status/log reads hang; GetMe502 propagates through the existing root error boundary, with no routing code change
- 261007 T647 done on main: sibling destinations, canonical retained test records and validated named returns; full FE4108/4109 plus corrected owning11, lint/build and Chromium14 contexts/112 geometry checks pass; preserve unrelated T630 edits
- 261007 T630 resume on main after8da67a15: external T632 is committed; exclusive sequential implementation continues through T646 with per-task verification and commits
- 261007 T632 done on main: additive semantic origins and safe request projections; shared38 fixtures, FE223 and owning Go suites/build/type/codegen pass; T635 waits for overlapping claimed T630 request assembly
- 261007 T630 freshness: EDIT4/THEME29/TMPL25/GUIDE17/MODEL35 add origin/prompt/tag work assigned to T632-T645; referenced setting policies remain unchanged, bases refreshed
- 261007 T632 start on main: additive semantic-origin and safe inspection contracts; independent of claimed T630 settings/publication work; preserve unrelated edits and commit only this task before selecting another
- 261007 T630 start on main: named settings, shared durable AI/direct editing, domain publication and atomic model adoption; sequential task completion and commits resume
- 261007 final frontend correction: deterministic polling/notification clock from mount replaces the mixed real/fake timer fixture; queued and running history/picker feeds automatically refresh, retain terminal failures and stop requests; owning usePostList/usePosts/useJob11 tests, CI-mode4, ESLint/format and TypeScript build pass, with no production source change
- 261007 remote backend verification at6a944d1f: full Go tests, format, vet, build and deployment recovery checks pass — https://github.com/hetarho/postpilot/actions/runs/37603125798/job/112732066963
- 261007 remote deployment at6a944d1f: build and actual rollout/health/browser-CORS/R2-GET steps all pass, none skipped — https://github.com/hetarho/postpilot/actions/runs/37603126041
