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
| T036 | Billing foundation: schema, the append-only money ledger, the provider and rate adapters, the read RPC and the billing screen | BILLING ARCH QUOTA AUTH | T034 | todo |
| T037 | Register a payment method through the hosted card window and grant the 100-credit bonus once | BILLING QUOTA AUTH | T036 T030 | todo |
| T038 | Subscribe from a rung, renew on the anchor day, and drop to free the moment a renewal fails | BILLING QUOTA MARKETING AUTH ARCH | T037 | todo |
| T039 | Upgrade at once, schedule downgrades and term shortening for the next anchor, cancel and resume | BILLING QUOTA ARCH | T038 | todo |
| T040 | Buy credits at par at any time, and refund an untouched purchase within seven days | BILLING QUOTA ARCH | T037 | todo |
| T042 | The r4 mutation vocabulary and the body mutations | PUBLISH | T018 | blocked@260908 |
| T043 | Caret-relative image insertion, one-at-a-time upload and captions | PUBLISH | T042 | todo |
| T044 | The settings layer, and tags, category and visibility inside it | PUBLISH | T042 | todo |
| T045 | The commit fence: arming, one activation, and readback through the post-view URL | PUBLISH | T044 T043 | todo |
| T046 | Wiring the real publisher into the daemon | PUBLISH | T045 | todo |

## next
- implement-task T036 → T037 → T038 → T039 → T040
- T042 blocked on ONE live survey pass on a clean writer draft: does 문단 서식 변경 convert the caret's paragraph or its whole component when the component holds two or more paragraphs (same for 인용구), what does Enter from a converted block open, and how does the list toolbar behave there — the owner must discard the leftover dirty draft in the browser first, since navigating away from it raises a `beforeunload` dialog the driver surface cannot dismiss · T044 is claimable NOW (dep T042 is only for the shared plumbing, which has landed) · then T043 → T045 → T046 → T008, whose base must be re-read at PUBLISH@4 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes

## log
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
- 260908 T029 done
- 260908 T029 claimed (cx)
- 260908 T041 done
- 260908 STATE restored T029..T040 and the consumed AUTH ARCH MARKETING BILLING revisions after spec lint exposed their pre-existing omission
- 260908 T041 claimed (cx)
- 260908 T042 blocked — the r4 vocabulary, the order-checked plan, `Apply` for title/text/open_settings and the occlusion latch are shipped and green; heading·quote·list wait on one unobserved fact (what 문단 서식 변경 converts when the caret's component holds two paragraphs). WARN fixed two real driver bugs on the way: `body_end` could put the caret in the document TITLE (it is a .se-component inside .se-body), and it aimed at each box's CENTRE, which lands mid-text on a paragraph that fills its line
- 260908 create-task PUBLISH → T042..T046 (the r4 vocabulary and body mutations, caret-relative image insertion, the settings layer, the commit fence with the post-view readback, the daemon wiring); T019 is superseded and moved to done with every box unchecked because its acceptance was written against r3, and T008's dep moves T019→T046
- 260907 update-ssot PUBLISH r4 done — a `filling_settings` stage joins the progress list, the settings layer is a versioned step that occludes the editor so body and photos precede it (PUBLISH-37), readback observes the post through the account's post-view URL while still reporting the permalink, and the locator-derived caret positions an inserted image; the frame-scoped driver addition r3 was expected to need is NOT required
