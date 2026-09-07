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
| ARCH | 2 | 1 | ARCH-1✎ 5✎ 23✎ | 0 |
| AUTH | 2 | 1 | AUTH-1✎ 2✎ 3✎ 4✎ 5✎ 17✎ 30✎ 33+ 34+ 35+ 36+ 37+ 38+ 39+ 40+ 41+ | 0 |
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
| MARKETING | 3 | 2 | MARKETING-6✎ 11✎ 16+ | 0 |
| VIDEO | 1 | 1 | - | 1 |
| BILLING | 2 | 0 | all | 0 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T008 | End-to-end verification and the authorized live Naver smoke publish | PUBLISH MARKETING | T007 T046 | todo |
| T041 | The memo names the subject in prose, alt and caption | GEN | - | todo |
| T042 | The r4 mutation vocabulary and the body mutations | PUBLISH | T018 | blocked@260908 |
| T043 | Caret-relative image insertion, one-at-a-time upload and captions | PUBLISH | T042 | todo |
| T044 | The settings layer, and tags, category and visibility inside it | PUBLISH | T042 | todo |
| T045 | The commit fence: arming, one activation, and readback through the post-view URL | PUBLISH | T044 T043 | todo |
| T046 | Wiring the real publisher into the daemon | PUBLISH | T045 | todo |

## next
- create-task AUTH QUOTA ARCH MARKETING BILLING is the next wave (self-signup, the anchor window, the payment-method bonus, the whole payment surface); every base takes QUOTA@5 · THEME@4 · MARKETING@3 · ARCH@2 on claim, then create-task for the AUTH·BILLING wave (self-signup, the anchor window, the payment-method bonus, the whole payment surface)
- T042 blocked on ONE live survey pass on a clean writer draft: does 문단 서식 변경 convert the caret's paragraph or its whole component when the component holds two or more paragraphs (same for 인용구), what does Enter from a converted block open, and how does the list toolbar behave there — the owner must discard the leftover dirty draft in the browser first, since navigating away from it raises a `beforeunload` dialog the driver surface cannot dismiss · T044 is claimable NOW (dep T042 is only for the shared plumbing, which has landed) · then T043 → T045 → T046 → T008, whose base must be re-read at PUBLISH@4 · update-ssot PUBLISH for VIDEO-17 + TEMPLATE-39 after T008 closes
- implement-task T041 — one prompt-side change with no dep, independent of the PUBLISH chain and of the AUTH·BILLING wave

## log
- 260908 T042 blocked — the r4 vocabulary, the order-checked plan, `Apply` for title/text/open_settings and the occlusion latch are shipped and green; heading·quote·list wait on one unobserved fact (what 문단 서식 변경 converts when the caret's component holds two paragraphs). WARN fixed two real driver bugs on the way: `body_end` could put the caret in the document TITLE (it is a .se-component inside .se-body), and it aimed at each box's CENTRE, which lands mid-text on a paragraph that fills its line
- 260908 create-task PUBLISH → T042..T046 (the r4 vocabulary and body mutations, caret-relative image insertion, the settings layer, the commit fence with the post-view readback, the daemon wiring); T019 is superseded and moved to done with every box unchecked because its acceptance was written against r3, and T008's dep moves T019→T046
- 260907 update-ssot PUBLISH r4 done — a `filling_settings` stage joins the progress list, the settings layer is a versioned step that occludes the editor so body and photos precede it (PUBLISH-37), readback observes the post through the account's post-view URL while still reporting the permalink, and the locator-derived caret positions an inserted image; the frame-scoped driver addition r3 was expected to need is NOT required
- 260907 T019 live survey done on the owner's Mac — all three blockers answered: an image inserts immediately after the caret's component (no placeholder, no pre-allocated ordinal), the settings layer takes TWO clicks and OCCLUDES the body so body mutations must come first, and `PostView.naver` serves a published post in its own document in the editor's own `.se-component` vocabulary, so readback needs no frame-scoped driver surface. Also found: `Enter` appends a paragraph INSIDE one `se-text` component, and the list control exists only while the caret is in a text block. WARN an unsaved scratch draft with one survey quote, paragraph and uploaded image is left in the writer — discard it, do not publish
- 260907 create-task GEN → T041 (the naming rule as two per-language constants beside the grounding ones, write prompt only; the revise golden stays byte-identical and the observe prompt is untouched)
- 260907 update-ssot GEN r3 done — the memo names the subject in prose, alt and caption (GEN-44); the observe stage stays context-free (GEN-45x) ← the caption already rides the write call that holds the memo (prompts.go perPost), so the gap was naming authority, not a missing input
- 260907 T007 done — ARCH-27 passed on the owner's Mac and CI's macOS agent job is green; the red backend job on main was T021's fixed-width timestamp parse (one run in ten), fixed in 4a923a6 by reading with RFC3339Nano like the post and auth stores. T019 stays blocked on the live survey; T008 waits behind it
- 260907 T028 done — the four rungs and the estimator wear a rotating accent-gradient stroke, transform-only and frozen still under reduced motion; the estimator wave is complete
- 260907 T028 claimed (pw)
- 260907 T027 done — /plans answers "몇 편" from three sliders and a combo switch, with no request per change; free 4 · basic 19 · pro 52 · max 108 at the default case
- 260907 T027 claimed (pw)
- 260907 T026 done — the 모델 관리 tab assigns the two models behind each of the four estimator combos, drafted locally and sent as one complete pair
- 260907 T026 claimed (pw)
- 260907 T025 done — four operator-assigned combos price a post by photo·video·1000 chars in milli-credits, published through GetMyPlan; the worst-case reference post is gone
- 260907 T025 claimed (pw)
- 260907 create-task QUOTA THEME → T025..T028 (combos + published rates, the operator's assignment, the /plans calculator, the animated promotional stroke); T023's worst-case reference post and its copy are removed in T025/T027
- 260907 update-ssot QUOTA r5 THEME r4 done (the post estimate becomes proportional over adjustable characters·photos·videos across four operator-assigned combos, and a promotional surface may animate a gradient stroke on every option)
- 260907 WARN T023 shipped the worst-case 32-credit reference post and its caveat copy — QUOTA-36 r5 replaces both, so create-task must plan the removal, not just an addition
- 260907 T024 done — /about no longer claims a plan decides daily job counts or a model range, and its figures match the raised ladder; a claim-level assertion now guards the sentence
- 260907 T024 claimed (pw), base MARKETING@2→@3 QUOTA@3→@4 LANG@1 ARCH@1→@2: MARKETING r3 binds the access sentence and the CTA to self-signup SHIPPING, which it has not, so this task still writes the operator path
