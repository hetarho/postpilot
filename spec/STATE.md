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
| VOICE | 15 | 15 | - | 0 |
| GEN | 26 | 26 | - | 0 |
| MODEL | 35 | 35 | - | 0 |
| TMPL | 25 | 25 | - | 0 |
| GUIDE | 17 | 17 | - | 0 |
| EXPORT | 11 | 11 | - | 0 |
| LANG | 9 | 9 | - | 0 |
| THEME | 30 | 30 | - | 1 |
| MKT | 10 | 10 | - | 0 |
| VIDEO | 7 | 7 | - | 0 |
| CLIP | 59 | 59 | - | 2 |
| CDS | 33 | 33 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 7 | 7 | - | 2 |
| QUAL | 8 | 8 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| EDIT | 4 | 4 | - | 0 |
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
| T627 | Prepare frozen single-factor inputs and one complete post per test entrant | ARCH GEN MODEL TMPL GUIDE LANG VIDEO QUOTA | T622 T625 T626 T630 | todo |
| T628 | Run private binary tournaments with exact metering and explicit winner publication | ARCH MODEL QUOTA GEN LANG | T622 T624 T625 T626 T627 T630 | todo |
| T630 | Show named setting states and integrate direct AI editing and real-writing tests | ARCH EDIT THEME TMPL GUIDE MODEL | T622 T625 | todo |
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
- T635 is dependency-ready after T632 but overlaps claimed T630 authoring request assembly; resume after T630 commits. Editorial follow-up: doc-review ARCH.
- T632 contracts are complete; runtime origin production, storage and request capture remain assigned to their dependent tasks.
- T603/T604 real semantic/hardware/release qualification stays blocked; no renderer, analysis, voice or distribution activation is authorized by this merge.

## log
- 261007 T647 done on main: sibling destinations, canonical retained test records and validated named returns; full FE4108/4109 plus corrected owning11, lint/build and Chromium14 contexts/112 geometry checks pass; preserve unrelated T630 edits
- 261007 T632 done on main: additive semantic origins and safe request projections; shared38 fixtures, FE223 and owning Go suites/build/type/codegen pass; T635 waits for overlapping claimed T630 request assembly
- 261007 T632 start on main: additive semantic-origin and safe inspection contracts; independent of claimed T630 settings/publication work; preserve unrelated edits and commit only this task before selecting another
- 261007 final frontend correction: deterministic polling/notification clock from mount replaces the mixed real/fake timer fixture; queued and running history/picker feeds automatically refresh, retain terminal failures and stop requests; owning usePostList/usePosts/useJob11 tests, CI-mode4, ESLint/format and TypeScript build pass, with no production source change
- 261007 remote backend verification at6a944d1f: full Go tests, format, vet, build and deployment recovery checks pass — https://github.com/hetarho/postpilot/actions/runs/37603125798/job/112732066963
- 261007 remote deployment at6a944d1f: build and actual rollout/health/browser-CORS/R2-GET steps all pass, none skipped — https://github.com/hetarho/postpilot/actions/runs/37603126041
- 261007 remote media verification at6a944d1f: image, worker and both-layout release jobs all pass — https://github.com/hetarho/postpilot/actions/runs/37603618318
- 261007 remote CI frontend failure at6a944d1f: usePostList retained-terminal polling test expected failed but saw running in picker; diagnose real observer/timer ordering before correcting it, preserve both-list refresh and stopped-poll assertions; remote deployment/release/worker gates pass and image/backend checks continue
- 261007 final remote verification start at6a944d1f: user requests remaining CI/deployment/media checks to finish before final verification record commit/push; no further implementation task is started
- 261007 post-push verification complete: whole Go run passed every package except the corrected rpcserver reason scan; rpcserver/authoring/authoring-rpc full owning suites, vet/build and format pass after genuine missing-capability/private-reason fixes; whole frontend failures corrected with owning-suite passes, generated-code drift/spec checks and production build pass; deployment recovery62 and all production/worker/both-layout media gates pass
- 261007 final backend correction scope: unsupported durable authoring adapter returns typed existing unavailable314 after authentication; reason scanner recognizes actual private failure fields without enum exemptions; media/worker execution paths unchanged and real release API adapter implements durable drafts, so compatible runtime gate evidence is retained and final deployable image metadata is rebuilt for pushed HEAD
- 261007 post-push frontend verification: full suite3992/4004 passed; the12 breadcrumb selector failures are corrected and owning suites pass (Admin26, Templates7); ESLint, format, FSD, style, retirement and production build pass; backend/media verification continues
- 261007 post-push corrections: preserve normalized voice test factor and safe filtered paid-history return; canonical consumers64, Admin catalog26, parser10 and migration compatibility11 cases pass; full FE/BE/media verification remains active
- 261007 main pushed normally through5edb107a after user requested push-first ordering; origin/local synchronized, deployment workflow success observed, initial CI FE/BE test failures retained for reproduction and correction
- 261007 main merge corrections complete: published browser148/wait149 preserved, local creation/writing schemas moved to150/151 with recognized legacy-schema reconciliation; full verification deferred until after push by user request
- 261007 user push-order override: stop pre-push tests, finish critical merge corrections and push first; run remaining verification afterward, preserving known failed legacy consumer checks for correction
- 261007 main merge reconciled: both UX and browser-media contributions retained, completed T590-T602 archived, T603/T604 remain blocked; resolve overlapping unpublished migration numbers before push
- 261007 main synchronization start: merge published origin/main browser-media history with local completed UX history, preserve both commit graphs and stopped task scope, run final-candidate pre-push checks before ordinary push
- 261007 T626 done on main: editable private sources and accepted snapshots, explicit estimated reanalysis, exact binary style batches and safe publication; owning/consumer tests, builds, codegen and lint checks pass; stopped as requested without starting another task
- 261007 user scope limited to current T626: finish implementation, verification and main commit, then stop without starting a next task
