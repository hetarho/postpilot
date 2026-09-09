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
| T045 | The commit fence: arming, one activation, and readback through the post-view URL | PUB | T044 T043 | doing@260910.fnc |
| T046 | Wiring the real publisher into the daemon | PUB | T045 | todo |
| T076 | Durable clip generation and result retention | CLIP QUOTA VIDEO MODEL | T071 T074 T075 | todo |
| T077 | Clip generation progress, preview and download | CLIP THEME LANG | T072 T076 | todo |
| T078 | Manual clip-plan save and credit-free rerender | CLIP QUOTA | T071 T074 T076 | todo |
| T079 | Clip correction workspace | CLIP THEME LANG | T077 T078 | todo |
| T080 | Grouped writing and video navigation | CLIP THEME | T070 T072 | todo |

## next
- Implement and commit T076 through T080 sequentially; T074's three ratios are owner-verified in Naver web
- spec maintenance owed: update-ssot VOICE for VOICE-42's frozen tag-count wording, update-ssot TMPL for the retired SLOT `label` vs `<ask label>` conflict, and update-ssot LANG to add the `clips` namespace
- the PUB body path is done and live-verified (T042 T043): read BOTH results before T045 — the 260910 surveys found that paragraph conversions split the component, 인용구 is born with a 출처 module, an image splits the caret's component and so needs no index, the photo-library sidebar overlays every caret point, and `Input.insertText` leaves its last word uncommitted until a key follows it (4 of 5 writes lost it). Next is T045 → T046 → T008 (re-read PUB@4 base); update-ssot PUB for VIDEO-17 + TMPL-39 after T008; T045 inherits the build-tagged survey harness in `agent/internal/naver/survey_test.go` for its own live checks, and must keep `image_caption`'s document-ordinal assumption true when it counts strip-grouped images
## log
- 260910 T075 done; strict timecoded AI contracts, safe caption placement/exposure, typed budgets and all local gates pass
- 260910 T075 claimed (clp)
- 260910 T074 done; exact caption/video renderer, every local gate and nonroot Docker smoke pass; owner confirms Naver web acceptance of all three ratios
- 260910 T045 claimed (fnc)
- 260910 T074 reclaimed (clp): owner confirms web acceptance of all three MP4s with a screenshot showing their 15-second durations; final freshness and gates before commit
- 260910 T043 done; photos interleave at their manifest positions, and four live defects fixed — the photo-library sidebar occluding every caret point, points read outside the viewport, an upload returning before the editor settled, and insertText leaving its last word uncommitted (4 of 5 writes lost it); driver signature → smarteditor-one-20260910-a5
- 260910 T074 gate rechecked (clp): owner picker result is still absent; preserve blocked status and the sequential commit boundary
- 260910 T074 blocked: all local gates and actual nonroot renderer smoke pass; mandatory owner-assisted Naver picker acceptance remains unverified, so no completion commit
- 260910 T074 claimed (clp)
- 260910 T073 done; bounded media adapter and real nonroot Docker smoke pass, dev media version matches
- 260910 T073 claimed (clp)
- 260910 T072 done; clip setup, page-local direct uploads and all local gates pass
- 260910 T043 claimed (img)
- 260910 T072 claimed (clp)
- 260910 T071 done; retryable transient-source cleanup, conditional direct upload and all local gates pass
- 260910 T042 done; the four blocked questions answered by a live survey, the body plan is now two passes with the conversions reversed, driver signature → smarteditor-one-20260910-a4
- 260910 T071 claimed (clp)
- 260910 T070 done; UI/contract/CI gates pass; pre-existing 320 px header wordmark/plan overlap noted for later shell review
- 260910 T042 claimed (bdy); the live survey answered all four questions that blocked it
- 260910 T070 claimed (clp)
