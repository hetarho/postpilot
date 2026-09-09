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
| TEMPLATE | 4 | 4 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUBLISH | 4 | 4 | - | 0 |
| LANG | 2 | 2 | - | 0 |
| THEME | 8 | 8 | - | 0 |
| MARKETING | 4 | 4 | - | 0 |
| VIDEO | 1 | 1 | - | 1 |
| CLIP | 1 | 1 | - | 0 |
| BILLING | 4 | 4 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 T046 | todo |
| T042 | The r4 mutation vocabulary and the body mutations | PUBLISH | T018 | blocked@260908 |
| T043 | Caret-relative image insertion, one-at-a-time upload and captions | PUBLISH | T042 | todo |
| T045 | The commit fence: arming, one activation, and readback through the post-view URL | PUBLISH | T044 T043 | todo |
| T046 | Wiring the real publisher into the daemon | PUBLISH | T045 | todo |
| T069 | Clip and video-template domain foundation | CLIP | - | blocked@260910 |
| T070 | Video-template management UI | CLIP THEME | T069 | todo |
| T071 | Transient clip-source batches and cleanup | CLIP | T069 | todo |
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
- unblock T069: implementation and code gates pass; existing spec ID lint failures need an owner exception or identifier migration; then implement and commit T070 through T080 sequentially
- spec maintenance owed: update-ssot VOICE for VOICE-42's frozen tag-count wording, update-ssot TEMPLATE for the retired SLOT `label` vs `<ask label>` conflict, update-ssot LANG to add the `clips` namespace, and resolve the existing overlong PUBLISH BILLING TEMPLATE MARKETING ids
- the PUBLISH chain stays as it was: unblock T042 with ONE live survey pass on a clean writer draft (does 문단 서식 변경 convert the caret's paragraph or its whole component on a multi-paragraph component, same for 인용구, what Enter from a converted block opens, how the list toolbar behaves there — the owner must discard the leftover dirty draft first), then T043 → T045 → T046 → T008, whose base must be re-read at PUBLISH@4 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes
## log
- 260910 T069 blocked: code gates pass; mandatory spec lint rejects existing TEMPLATE/PUBLISH/MARKETING/BILLING ids; owner exception requested
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
- 260909 create-task POST THEME start
- 260909 update-ssot POST r5 THEME r8 (POST-43✎ 64+ 65+ 66+ 67+ 68+ 69+ · THEME-24✎ a list's add dock is no longer phone-only); no doing task in scope (T065 is MODEL) — create-task POST THEME owed
- 260909 update-ssot POST THEME start (post list: 새 글 as a dock at every width, filter + search by title/tag)
- 260909 create-ssot CLIP start
- 260909 T065 claimed (blk)
- 260909 update-ssot MODEL r8 (MODEL-53✎); no doing task in scope, T065 (todo) unaffected — its ssot is MODEL-52 54 55 56
- 260909 update-ssot MODEL start (MODEL-53 reject causes vs what the context can see)
- 260909 T064 done
