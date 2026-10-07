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
| ARCH | 19 | 18 | ARCH-24✎ ARCH-25✎ ARCH-26✎ ARCH-31✎ ARCH-37✎ | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 38 | 38 | - | 0 |
| POST | 36 | 35 | POST-63✎ POST-112+ POST-113+ POST-114+ POST-115+ | 0 |
| VOICE | 15 | 14 | VOICE-75+ | 0 |
| GEN | 26 | 25 | GEN-14✎ GEN-16✎ GEN-40✎ GEN-41✎ GEN-46✎ GEN-49✎ GEN-50✎ GEN-68✎ GEN-70✎ GEN-73✎ GEN-77✎ GEN-80+ GEN-81+ GEN-82+ GEN-83+ GEN-84+ GEN-85+ GEN-86+ | 0 |
| MODEL | 35 | 34 | MODEL-5✎ MODEL-32✎ MODEL-37✎ MODEL-42✎ MODEL-93+ MODEL-94+ MODEL-95+ | 0 |
| TMPL | 25 | 24 | TMPL-21✎ TMPL-26✎ TMPL-47✎ TMPL-49✎ TMPL-70+ | 0 |
| GUIDE | 17 | 16 | GUIDE-1✎ GUIDE-15✎ GUIDE-37✎ GUIDE-41✎ GUIDE-50+ | 0 |
| EXPORT | 11 | 10 | EXPORT-27+ | 0 |
| LANG | 9 | 8 | LANG-29+ | 0 |
| THEME | 29 | 28 | THEME-62+ THEME-63+ | 1 |
| MKT | 10 | 9 | MKT-17+ | 0 |
| VIDEO | 7 | 7 | - | 0 |
| CLIP | 59 | 59 | - | 2 |
| CDS | 33 | 33 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 7 | 6 | MEM-22✎ MEM-31+ | 2 |
| QUAL | 8 | 7 | QUAL-6✎ QUAL-48+ | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| EDIT | 4 | 3 | EDIT-24+ | 0 |
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
| T623 | Expose current location parent access and contextual creation return | ARCH THEME CLIP MODEL | T622 T629 | todo |
| T624 | Make writing spacious and replace archive browsing with operational history | ARCH THEME POST CLIP GEN | T622 | todo |
| T625 | Unify named AI and direct editing with bounded seed-free candidate preparation | ARCH EDIT THEME MODEL QUOTA | T622 | todo |
| T626 | Edit learning materials while preserving accepted voice profiles and test provenance | ARCH VOICE MODEL QUOTA | T622 T625 | todo |
| T627 | Prepare frozen single-factor inputs and one complete post per test entrant | ARCH GEN MODEL TMPL GUIDE LANG VIDEO QUOTA | T622 T625 T626 T630 | todo |
| T628 | Run private binary tournaments with exact metering and explicit winner publication | ARCH MODEL QUOTA GEN LANG | T622 T624 T625 T626 T627 T630 | todo |
| T629 | Build one human A/B and knockout test experience for reusable writing settings | ARCH THEME MODEL QUOTA | T622 | todo |
| T630 | Show named setting states and integrate direct AI editing and real-writing tests | ARCH EDIT THEME TMPL GUIDE MODEL | T622 T625 | todo |
| T631 | Integrate isolated UX bundles and qualify creation settings and sixteen-entry tests | ARCH THEME POST CLIP EDIT VOICE MODEL QUOTA | T622 T623 T624 T625 T626 T627 T628 T629 T630 | todo |

## next
- create-task ARCH; active creation-comparison-ux attempts must sync ARCH@19 verification stages before continuing the T622–T631 plan in docs/work/creation-and-comparison-ux.md; final visual arrangement remains THEME-61 open.
- Existing browser-media tasks/blocked qualifications retain their scope; shared touches must respect any active ownership.
- create-task GEN POST GUIDE TMPL THEME EXPORT MKT LANG VOICE MEM QUAL MODEL EDIT: consume the prompt-engineering deltas with existing T624–T631 ownership; see docs/work/prompt-engineering-ssot-2026-10-07.md. Instruction-language and live-model quality comparisons are validation, not open product policy.

## log
- 261007 main integration start: combine completed T624/T625/T629 with baseline/CAS/recovery corrections; preserve pending prompt-engineering requirements and validate the combined code
- 261007 update-ssot prompt-engineering verified: spec lint exit0 with124 history/freshness warnings and13 editorial hints; thirteen revisions and51 decision deltas match STATE/chg, historical IDs/references/links/log checks and diff check pass; tasks/product code unchanged
- 261007 update-ssot prompt-engineering affects active creation-comparison-ux attempts T624/17671fdd changes_requested, T625/2182ddca changes_requested and T629/7c124aa5 ready: synchronize/reassess affected deltas before submission/integration; existing checks do not verify new semantic-origin behavior
- 261007 update-ssot prompt-engineering done: thirteen domains revised; phrase origins, visible AI expression, no photo-order chronology, maximum grounded tags and safe prompt inspection fixed; converted ideation has no open product decisions, existing unrelated open items retained
- 261007 update-ssot GEN POST GUIDE TMPL THEME EXPORT MKT LANG VOICE MODEL QUAL MEM EDIT start: convert prompt efficiency and owner-controlled writing decisions; reconcile three-source review, visible AI expression and photo chronology with current policies
- 261007 ideation prompt-engineering owner-control round recorded: scope corrected; three semantic sources, expressive assistance and photo-order chronology explored with an interactive synthetic mockup; granularity/addition policy pending, SSOT/tasks unchanged
- 261007 ideation prompt-engineering owner-control round start: correct scope to context efficiency/maintainability plus writing/tag quality; explore three-source text review, AI-assisted expression and photo-order-independent storytelling before SSOT/task conversion
- 261007 ideation prompt-engineering deep review recorded: thirty-one audit candidates and twenty primary-source links; twelve existing checks plus five synthetic diagnostics, reference-token measurements and independent reviews; language choice remains open, tag default ownership confirmed
- 261007 ideation prompt-engineering deep review start: audit prompt structure, length and contradictions across stages; research Korean versus English instructions with Korean output and separate evidence from hypotheses
- 261007 ideation prompt-engineering round recorded: current prompt map and 995bcee9 tag path traced; thirteen existing tests pass; both inspection audiences and up-to-N grounded tags chosen, refactor scope remains open
- 261007 ideation prompt-engineering start: improve writing quality before owner-observed view trials; trace actual prompt composition and the recent tag change, then explore a comprehensive prompt refactor
- 261007 ideation searchable-details research round recorded: 64 linked sources, official 2026 content guides and search changes checked; numeric SEO claims assessed, three first-benefit candidates remain open
- 261007 ideation searchable-details research start: recheck Naver Blog search principles, recent search changes and practitioner SEO claims; product choices remain open
- 261007 create-architecture ARCH r19 done: task-impact checks at completion, full CI and applicable backend/media gates before push; local agent instructions and runnable parity runbook added, remote failure logs unavailable
- 261007 ARCH r19 affects active creation-comparison-ux/T622 attempt and pending ARCH tasks: sync/reassess the verification policy before further submission or integration; no worker workspace changed
- 261007 create-architecture ARCH start: separate task impact verification from pre-push CI/CD checks and inspect backend/media workflow failures
- 261007 create-task creation/comparison UX done: T622–T631 form ten atomic session bundles with touches/dependency ownership; thirteen SSOT deltas consumed and visual decision THEME-61 remains open
- 261007 manage-work planning recorded: native task claims are bundle claims; scope-filter T622–T631 to avoid unrelated media work, workers10 configured only when work group is started
- 261007 update-ssot creation/comparison UX done: thirteen domains revised, human winners and once-per-entrant generation fixed; final visual arrangement stays THEME-61 open
- 261007 create-task ARCH THEME POST CLIP EDIT VOICE TMPL GUIDE MODEL GEN QUOTA LANG VIDEO start: ten atomically claimable session bundles with isolated touches and explicit integration gates
- 261007 update-ssot THEME POST CLIP AUTH EDIT VOICE TMPL GUIDE MODEL GEN QUOTA start: review contextual navigation, operational history, understandable setting drafts, editable learning material and unified binary tournaments; unresolved product choices stay open
