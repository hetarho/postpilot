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
| AUTH | 2 | 2 | - | 0 |
| QUOTA | 5 | 5 | - | 0 |
| POST | 2 | 2 | - | 0 |
| VOICE | 1 | 1 | - | 1 |
| GEN | 3 | 3 | - | 0 |
| MODEL | 3 | 3 | - | 0 |
| TEMPLATE | 3 | 3 | - | 1 |
| GUIDE | 1 | 1 | - | 0 |
| EXPORT | 2 | 2 | - | 0 |
| PUBLISH | 4 | 4 | - | 0 |
| LANG | 1 | 1 | - | 0 |
| THEME | 4 | 4 | - | 0 |
| MARKETING | 3 | 3 | - | 0 |
| VIDEO | 1 | 1 | - | 1 |
| BILLING | 2 | 2 | - | 0 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 T046 | todo |
| T040 | Buy credits at par at any time, and refund an untouched purchase within seven days | BILLING QUOTA ARCH | T037 | todo |
| T042 | The r4 mutation vocabulary and the body mutations | PUBLISH | T018 | blocked@260908 |
| T043 | Caret-relative image insertion, one-at-a-time upload and captions | PUBLISH | T042 | todo |
| T044 | The settings layer, and tags, category and visibility inside it | PUBLISH | T042 | todo |
| T045 | The commit fence: arming, one activation, and readback through the post-view URL | PUBLISH | T044 T043 | todo |
| T046 | Wiring the real publisher into the daemon | PUBLISH | T045 | todo |

## next
- implement-task T040
- T042 blocked on ONE live survey pass on a clean writer draft: does 문단 서식 변경 convert the caret's paragraph or its whole component when the component holds two or more paragraphs (same for 인용구), what does Enter from a converted block open, and how does the list toolbar behave there — the owner must discard the leftover dirty draft in the browser first, since navigating away from it raises a `beforeunload` dialog the driver surface cannot dismiss · T044 is claimable NOW (dep T042 is only for the shared plumbing, which has landed) · then T043 → T045 → T046 → T008, whose base must be re-read at PUBLISH@4 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes

## log
- 260908 T039 done
- 260908 T039 claimed (cx)
- 260908 T038 done
- 260908 T038 claimed (cx)
- 260908 T037 done
- 260908 T037 claimed (cx)
- 260908 T036 done
- 260908 T036 claimed (cx)
- 260908 T035 done
- 260908 T035 claimed (cx)
- 260908 T033 done
- 260908 T033 claimed (cx)
- 260908 T032 done
- 260908 T032 claimed (cx)
- 260908 T031 done
- 260908 T031 claimed (cx)
- 260908 T030 done
- 260908 T030 claimed (cx)
- 260908 T034 done
- 260908 T034 claimed (cx)
