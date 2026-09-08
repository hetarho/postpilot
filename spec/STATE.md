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
| QUOTA | 6 | 6 | - | 0 |
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
| T054 | Every backend failure reason the browser can be handed | ARCH | - | todo |

## next
- implement-task T054 — the last of the eight review-sourced tasks
- spec-wide, out of scope for any of them: `haeram-spec-creator lint` rejects PUBLISH BILLING TEMPLATE MARKETING against FORMAT's `2-6 uppercase` id rule — either the rule or the four domain ids has to move, and it is a planning decision
- the PUBLISH chain stays as it was: unblock T042 with ONE live survey pass on a clean writer draft (does 문단 서식 변경 convert the caret's paragraph or its whole component on a multi-paragraph component, same for 인용구, what Enter from a converted block opens, how the list toolbar behaves there — the owner must discard the leftover dirty draft first), then T043 → T045 → T046 → T008, whose base must be re-read at PUBLISH@4 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes

## log
- 260908 T053 done
- 260908 T053 claimed (op5)
- 260908 T052 done
- 260908 T052 claimed (op5)
- 260908 T051 done
- 260908 T051 claimed (op5)
- 260908 T050 done
- 260908 T050 claimed (op5)
- 260908 T049 done
- 260908 T049 claimed (op5)
- 260908 T048 done
- 260908 T048 claimed (op5)
- 260908 T047 done
- 260908 T047 claimed (op5)
- 260908 create-task T047-T054 from BILLING@4 QUOTA@6 review/diff-260908
- 260908 create-task BILLING review/diff-260908 start
- 260908 update-ssot BILLING r3 r4 · QUOTA r6 (QUOTA-42+)
- 260908 update-ssot BILLING start
- 260908 review-code diff-260908 ready: F1-F8 F13 adopted, F9-F12 held
- 260908 review-code diff-260908 start
