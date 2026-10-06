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
| THEME | 26 | 26 | - | 0 |
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
| T590 | Bound Max server-render admission and waiting | ARCH CLIP INFRA | T589 | blocked@261006 |
| T591 | Freeze one browser composition and time contract | ARCH CLIP CDS | T588 | todo |
| T592 | Decode selected video ranges in a bounded browser pipeline | ARCH CLIP CDS | T591 | todo |
| T593 | Bound selected audio and immutable narration processing | ARCH CLIP CDS DUB | T591 | todo |
| T594 | Draw bundled typography and static components locally | ARCH CLIP CDS | T591 | todo |
| T595 | Animate caption transforms and masks from output time | ARCH CLIP CDS | T594 | todo |
| T596 | Port caption blur, light, colour and glitch effects | ARCH CLIP CDS | T595 | todo |
| T597 | Render ember caption geometry and particles locally | ARCH CLIP CDS | T596 | todo |
| T598 | Measure caption backgrounds from local original frames | ARCH CLIP CDS | T592 T594 | todo |
| T599 | Stream browser output and promote the verified private result | ARCH CLIP CDS | T592 T593 T595 T596 T597 T598 | todo |
| T600 | Use the browser composition engine throughout editing previews | ARCH CLIP CDS | T592 T593 T594 T595 T596 T597 T598 | todo |
| T601 | Prepare bounded AI analysis copies in the browser | ARCH CLIP CDS | T592 T602 | todo |
| T602 | Authorize and verify browser-prepared analysis artifacts | ARCH CLIP QUOTA | T591 | todo |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | todo |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | todo |
| T617 | Make XState actors the authority for guided creation workflows | ARCH THEME AUTH VOICE EDIT | - | doing@261007.ux |
| T618 | Guide writing voice learning through one purposeful step at a time | VOICE AUTH THEME | T617 | doing@261007.ux |
| T619 | Separate AI authoring into purpose choice review and optional refinement | EDIT THEME | T617 | doing@261007.ux |
| T620 | Integrate and qualify the research-backed guided design language | ARCH THEME AUTH VOICE EDIT | T617 T618 T619 | doing@261007.ux |

## next
- Implement T617 actors, T618 personal learning and T619 staged authoring; integrate and qualify T620.
- Commit each task independently; preserve existing browser-media assignments and dirty work.

## log
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
- 261007 T616 start: root integrates default AI template/guideline/voice entrypoints while isolated backend and Studio proposals proceed
- 261007 T613-T615 start: root owns main/codegen/commits; isolated core, publication and Studio proposals execute in parallel
- 261007 create-task EDIT and related deltas done: T613 durable drafts, T614 domain publication, T615 shared Studio, T616 entrypoints/qualification
- 261007 create-ssot EDIT r1 and update-ssot complete: durable eight-suggestion chat drafts, explicit guarded domain publication and responsive shared settings/setup UX
- 261007 update-ssot TMPL GUIDE VOICE CLIP THEME AUTH QUOTA start: eight AI suggestions, conversational draft editing and explicit settings adoption
- 261007 task commits: T606 be489e55, T607 9074f741, T608 fb856773, T609 a6c4a953, T610 a3003583, T611 a39ae36d; isolated staged-tree checks preserve existing workspace edits
- 261007 T611 done: 3383 FE tests, BE/build/tooling and 62 deploy tests pass; 80 live-browser layout checks and personal/AI/settings recovery flows pass
- 261007 T612 done: grounded targeted tags in both languages; generation/guideline tests, vet and production build pass; whole-tree build fails on existing tmp main/run duplicates
- 261007 T612 start: implement grounded entity and area/topic priority in both tag-rule languages
- 261007 create-task GEN r24 done: T612 updates the existing switchable tag rule and prompt goldens
