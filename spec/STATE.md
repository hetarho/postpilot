# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 2 | 2 | - | 0 |
| AUTH | 5 | 5 | - | 0 |
| QUOTA | 6 | 6 | - | 0 |
| POST | 5 | 5 | - | 0 |
| VOICE | 2 | 2 | - | 1 |
| GEN | 5 | 5 | - | 0 |
| MODEL | 8 | 8 | - | 0 |
| TMPL | 4 | 4 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUB | 4 | 4 | - | 0 |
| LANG | 2 | 2 | - | 0 |
| THEME | 8 | 8 | - | 0 |
| MKT | 4 | 4 | - | 0 |
| VIDEO | 1 | 1 | - | 1 |
| CLIP | 1 | 1 | - | 0 |
| BILL | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUB MKT | T007 T046 | todo |
| T043 | Caret-relative image insertion, one-at-a-time upload and captions | PUB | T042 | todo |
| T045 | The commit fence: arming, one activation, and readback through the post-view URL | PUB | T044 T043 | todo |
| T046 | Wiring the real publisher into the daemon | PUB | T045 | todo |
| T071 | Transient clip-source batches and cleanup | CLIP | T069 | doing@260910.clp |
| T072 | Clip setup and direct-upload UI | CLIP THEME | T070 T071 | todo |
| T073 | Clip media probe, analysis proxies and isolated workspaces | CLIP | T069 | todo |
| T074 | Deterministic caption and video renderer | CLIP | T073 | todo |
| T075 | Timecoded clip analysis and AI edit planning | CLIP VIDEO MODEL | T069 | todo |
| T076 | Durable clip generation and result retention | CLIP QUOTA VIDEO MODEL | T071 T074 T075 | todo |
| T077 | Clip generation progress, preview and download | CLIP THEME LANG | T072 T076 | todo |
| T078 | Manual clip-plan save and credit-free rerender | CLIP QUOTA | T071 T074 T076 | todo |
| T079 | Clip correction workspace | CLIP THEME LANG | T077 T078 | todo |
| T080 | Grouped writing and video navigation | CLIP THEME | T070 T072 | todo |

## next
- implement-task T071, then implement and commit T072 through T080 sequentially
- spec maintenance owed: update-ssot VOICE for VOICE-42's frozen tag-count wording, update-ssot TMPL for the retired SLOT `label` vs `<ask label>` conflict, and update-ssot LANG to add the `clips` namespace
- the PUB chain is unblocked: T042 is done and its `## result` carries the 260910 live survey (paragraph conversions split the component, 인용구 is born with a 출처 module, only a heading escapes by Enter, the list control needs a plain text paragraph) plus the two-pass body plan every later task inherits — read it before T043. Next is T043 → T045 → T046 → T008, whose base must be re-read at PUB@4 · update-ssot PUB for VIDEO-17 + TMPL-39 after T008 closes
- T043 needs ONE live pass for the one image mechanic never surveyed: a manifest whose FIRST block is an image, with no preceding block to hold the caret. The build-tagged survey harness in `agent/internal/naver/survey_test.go` is left in place for it
## log
- 260910 T042 done; the four blocked questions answered by a live survey, the body plan is now two passes with the conversions reversed, driver signature → smarteditor-one-20260910-a4
- 260910 T071 claimed (clp)
- 260910 T070 done; UI/contract/CI gates pass; pre-existing 320 px header wordmark/plan overlap noted for later shell review
- 260910 T042 claimed (bdy); the live survey answered all four questions that blocked it
- 260910 T070 claimed (clp)
- 260910 T069 done; all code gates and spec lint pass after identifier repair
- 260910 spec identifier repair: TEMPLATE→TMPL, PUBLISH→PUB, MARKETING→MKT, BILLING→BILL; names and references only, no policy or revision change
- 260910 T069 claimed (clp); owner authorized fixing spec lint and continuing through T080
- 260910 T069 blocked: code gates pass; mandatory spec lint rejects existing TMPL/PUB/MKT/BILL ids; owner exception requested
- 260909 T069 claimed (clp)
- 260909 create-task T069 T070 T071 T072 T073 T074 T075 T076 T077 T078 T079 T080 from CLIP@1 (foundation → templates/source/media/AI → generation/result → correction; grouped nav after both video pages)
- 260909 T068 done
- 260909 T068 claimed (srch)
- 260909 T067 done (the dock change also landed on /voices and /templates)
- 260909 T067 claimed (dock)
- 260909 T066 done
- 260909 T066 claimed (tag)
- 260909 create-task CLIP start
- 260909 create-task T066 T067 T068 from POST@5 THEME@8 (BE `PostSummary.tags` · the list dock at every width incl. /voices and /templates · the FE narrowing); T068 deps on T066 for the field and on T067 for the shared PostsPage.tsx
- 260909 create-ssot CLIP r1 (CLIP-1+ … 28+: independent projects, video templates, AI-selected cuts, correction, transient sources, result-only retention and no direct publishing) — create-task CLIP owed
