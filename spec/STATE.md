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
| creation-and-comparison-ux | open@261007 |
| storyline-first | converted@260927 |
| voice-tidy | converted@260929 |
| daily-credit-plans | converted@260929 |
| template-from-request | converted@261001 |
| familiar-video-editing-and-dubbing | converted@261004 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 18 | 18 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 38 | 38 | - | 0 |
| POST | 35 | 35 | - | 0 |
| VOICE | 14 | 14 | - | 0 |
| GEN | 25 | 25 | - | 0 |
| MODEL | 34 | 34 | - | 0 |
| TMPL | 24 | 24 | - | 0 |
| GUIDE | 16 | 16 | - | 0 |
| EXPORT | 10 | 10 | - | 0 |
| LANG | 8 | 8 | - | 0 |
| THEME | 28 | 28 | - | 1 |
| MKT | 9 | 9 | - | 0 |
| VIDEO | 7 | 7 | - | 0 |
| CLIP | 59 | 59 | - | 2 |
| CDS | 33 | 33 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 6 | 6 | - | 2 |
| QUAL | 7 | 7 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| EDIT | 3 | 3 | - | 0 |
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
| T622 | Freeze shared navigation authoring material and writing-test contracts | ARCH THEME POST EDIT VOICE MODEL QUOTA LANG | - | todo |
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
- Implement T622 first, then atomically claim an eligible T622–T631 session bundle using docs/work/creation-and-comparison-ux.md; final visual arrangement remains THEME-61 open.
- Existing browser-media tasks/blocked qualifications retain their scope; shared touches must respect any active ownership.

## log
- 261007 create-task creation/comparison UX done: T622–T631 form ten atomic session bundles with touches/dependency ownership; thirteen SSOT deltas consumed and visual decision THEME-61 remains open
- 261007 manage-work planning recorded: native task claims are bundle claims; scope-filter T622–T631 to avoid unrelated media work, workers10 configured only when work group is started
- 261007 update-ssot creation/comparison UX done: thirteen domains revised, human winners and once-per-entrant generation fixed; final visual arrangement stays THEME-61 open
- 261007 create-task ARCH THEME POST CLIP EDIT VOICE TMPL GUIDE MODEL GEN QUOTA LANG VIDEO start: ten atomically claimable session bundles with isolated touches and explicit integration gates
- 261007 update-ssot THEME POST CLIP AUTH EDIT VOICE TMPL GUIDE MODEL GEN QUOTA start: review contextual navigation, operational history, understandable setting drafts, editable learning material and unified binary tournaments; unresolved product choices stay open
- 261007 review-code desktop-ux-policy-261007 done: all-width menu policy, Sheet modal contract, prose-frame exception and missing writing-height/desktop checks traced;54 tests/style61 pass and six Chromium width/theme reproductions recorded
- 261007 review-code desktop-ux-policy-261007 start: trace navigation overlays, desktop composition and writing-area sizing against policy and acceptance checks
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
