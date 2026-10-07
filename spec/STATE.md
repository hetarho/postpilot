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
| familiar-video-editing-and-dubbing | converted@261004 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 17 | 17 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 37 | 37 | - | 0 |
| POST | 34 | 34 | - | 0 |
| VOICE | 13 | 13 | - | 0 |
| GEN | 24 | 24 | - | 0 |
| MODEL | 33 | 33 | - | 0 |
| TMPL | 23 | 23 | - | 0 |
| GUIDE | 15 | 15 | - | 0 |
| EXPORT | 10 | 10 | - | 0 |
| LANG | 7 | 7 | - | 0 |
| THEME | 27 | 27 | - | 0 |
| MKT | 9 | 9 | - | 0 |
| VIDEO | 6 | 6 | - | 0 |
| CLIP | 58 | 58 | - | 2 |
| CDS | 33 | 33 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 6 | 6 | - | 2 |
| QUAL | 7 | 7 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| EDIT | 2 | 2 | - | 0 |
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

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | todo |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | todo |

## next
- T603 and T604 remain runtime blocked; use the work board and their result/open gates before resuming qualification.
- Technical delivery through T604 is preserved on work/browser-media-604-plan; independent analysis, real voice and release activation stay disabled.

## log
- 261007 main merge candidate ready: current origin/main authoring/XState/typography preserved; forward149 upgrades and legacy144 preservation pass, actual phone/desktop sticky preview checks pass; full CI and authorized main push pending
- 261007 T604 source-only delivery preserved: independently reviewed2b5d5915 merged normally; local HTTPS13, paired Mac16 and portable4 checks pass, six release acceptances remain open and runtime blocked
- 261007 browser-media-604 local technical work closed through T604: task commits and scoped receipts preserved; T603 real semantic qualification and T604 source/hardware/human/performance release gates remain blocked, no live deployment or activation
- 261007 T600 integrated
- 261007 T599 integrated
- 261007 T603 source-only integration start: reviewed technical harness/AAC guard and actual pinned verifier proof; all six real qualification checks remain open and runtime blocked
- 261006 T597 integrated
- 261006 T601 integrated
- 261006 T596 integrated
- 261006 T598 integrated
- 261006 T602 integrated
- 261006 T595 integrated
- 261006 T590 integrated
- 261007 T590 resumed: isolated Colima CPU images and actual Docker generators recovered without interrupting Desktop/dev; independent audit reproduced pre-park accepted-output race for owned correction
- 261006 T593 integrated
- 261006 T594 integrated
- 261006 T592 integrated
- 261007 create-task native parity hints done: T593 source clock/preroll and T600 automatic geometry notes refined from existing native contracts; goals, acceptance, dependencies and SSOT remain unchanged
- 261007 create-task native parity hints start: unassigned T593/T600 require cumulative frame-aligned source audio and automatic caption geometry after edits; current worker contracts remain unchanged
- 261007 external checkout isolation: separate novice-UX work owns main changes and migration0145; analysis worker owns new migration0146, preserving both scopes without copying dirty main
- 261006 T591 integrated
- 261007 manage-work integration compatibility: temporary CLI preserves only46 inherited FORMAT/history warnings; structural errors and new warnings still reject; actual baseline/candidate conformance passed, installed package/skills/runtime JSON unchanged
- 261007 T591 independent native comparison correction: global pace/accent, declared disclosure only and exact-ms visibility require correction before integration; parent remains unchanged and prior approval invalidated
- 261007 main integration start: preserve the existing T590 draft separately; merge reviewed browser-media task commits with current origin/main, verify, and push under explicit user authorization
- 261007 media-release fixture fix done: Max support assignment passes colocated182.53s/remote159.11s CPU releases, BE80, deploy61, Go vet/build/gofmt and spec lint with existing warnings; verified default release image tags refreshed
- 261007 T621 done: typography b152f586 and final voice-context 4d634b14;3468 FE tests,145 browser measurements and all available unchanged-source/tooling gates pass
- 261007 create-task THEME r27 done; T621 start: shared type scale, focused role assignment and browser hierarchy verification
- 261007 update-ssot THEME r27 done: coherent responsive title/body scale and active-stage hierarchy
- 261007 update-ssot THEME start: responsive typography scale, active-step hierarchy and readable supporting copy
- 261007 T620 done: FE3468/430 files, clean258, backend80/deploy62/tooling/generator gates, twenty AI and four personal browser sessions pass;58 existing dirty files preserved
- 261007 T619 done: focused five-kind purpose/choice/review/optional-chat/publication actors;37 tests and112 day/night/mobile/desktop layout checks pass
- 261007 T618 done: method-specific personal learning, explicit analysis/use and readonly uncertain-job recovery; actor/API9 and host regressions77 pass
- 261007 T617 done: four real XState workflow authorities with invoked async fencing and frozen retries;60 actor/live-consumer tests pass
- 261007 task commits: T61799c13926, T618122ad5ac, T6191dd7a065, T620c3fd6d58; independent source commits plus final linked verification records
- 261007 T617-T620 start: root owns documentation/dependencies/commits; actor, personal-learning and authoring proposals have isolated file ownership
- 261007 create-task focused UX done: T617 XState authority, T618 personal funnel, T619 staged AI workspace, T620 shared design/integration
- 261007 ARCH r10..r16 allocation reconciled to existing T380/T386/T388 and T591-T604; scoped ARCH-69 allocated to T617/T620 without existing task rewrites
- 261007 update-ssot THEME r26 AUTH r15 VOICE r13 EDIT r2 and ARCH r17 done: focused method/collection/review/publication screens, contextual actions and XState authority
- 261007 update-ssot THEME AUTH VOICE EDIT and ARCH start: primary-source UX research, progressive task funnels and XState interaction ownership
- 261007 T616 done: all five AI settings/setup hosts, manual alternatives and personal learning preserved; FE3422/staged222, BE80 packages, deploy62 and fifteen browser flows pass
- 261007 T615 done: owner-scoped guarded eight-suggestion preview/chat Studio; responsive themes/focus/zoom and no-call entry/recovery pass
- 261007 T614 done: domain-owned atomic receipt publication, CAS/protected fields, concurrent replay/tombstones and synthetic-only voice forks pass
- 261007 T613 done: durable five-kind sessions, request replay, frozen bounded calls, revision/account fences and interrupted-save recovery pass24 core/RPC/SQLite tests
- 261007 task commits: T613 e8335bb1, T614 199dd222, T615 d98cca13, T616 4b9e6f5a; existing render-capacity source bytes preserved
