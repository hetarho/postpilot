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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 9 | 9 | - | 0 |
| AUTH | 8 | 8 | - | 0 |
| QUOTA | 19 | 19 | - | 0 |
| POST | 14 | 13 | POST-51✎ POST-54✎ POST-71✎ POST-81✎ POST-82✎ POST-89+ | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 11 | 11 | - | 0 |
| MODEL | 17 | 17 | - | 0 |
| TMPL | 11 | 11 | - | 1 |
| GUIDE | 6 | 6 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 18 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 43 | 40 | CLIP-13✎ | 1 |
| CDS | 25 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 3 | 2 | MEM-18✎ MEM-26✎ | 2 |
| QUAL | 4 | 4 | - | 0 |

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

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T354 | A generate's payload is encoded and decoded inside generation | ARCH | - | todo |
| T355 | Saving one generation option never touches another | POST ARCH | - | todo |
| T356 | Tab moves through an anchored panel; a hover-opened panel closes on Escape | THEME ARCH | T355 | todo |
| T357 | The published lock is one guard, and an unclassified write fails a test | ARCH | T355 | todo |
| T358 | A photo's row goes before its object, and "finalized at the current revision" is one rule | ARCH | T357 | todo |
| T359 | The write prompt builder and template Create each take one named input | ARCH | T354 | todo |
| T360 | Ticked quality rules open the per-post half; in revise the title form binds only a title request | GEN TMPL ARCH | T359 | todo |
| T361 | A write comparison freezes the same write material as Start | MEM MODEL GEN GUIDE QUAL ARCH | T354 T359 | todo |
| T362 | One replacement-rule fixture both sides read | ARCH | - | todo |
| T363 | A taken replacement candidate is spent | GEN POST ARCH | T355 T362 | todo |
| T364 | A measurement change cannot ship without its version bump, and emoji tails trim | ARCH | - | todo |
| T365 | Generation preconditions take one named input | ARCH | - | todo |
| T366 | The draft queue's assignments are one record per channel | ARCH | - | todo |
| T367 | The autosave owns the published lock, and every control reads the lock from the post it holds | ARCH | T355 T365 T366 | todo |
| T368 | The editor page suites pin behavior, not wiring | ARCH | T355 T356 T363 T365 T367 | todo |
| T369 | An empty search answer keeps the stored list, and the refresh interval has a floor | QUAL ARCH | T364 | todo |
| T370 | Quality reads the post it needs and nothing more, and TopNoun is gone | ARCH | T357 T364 | todo |
| T371 | The small mirrors are pinned | ARCH | T362 T364 T369 | todo |
| T372 | The test harness pins what it claims, and template limits have one constructor | ARCH | T354 T355 T359 T369 T371 | todo |
| T373 | Entity boundaries for 분야 and post status | ARCH | T356 T367 | todo |
| T374 | The template screen's ask-conflict flags follow the mounted composition | ARCH | - | todo |
| T375 | The editor's per-control cases live in the tests of the slices that own them | ARCH | T368 | todo |

## next
- create-task POST MEM first: 분야 and 기억 사용 move into the writing brief, saved together by its 저장 — rewrite todo T355 and re-check T366 T367 T368 T373 T375, which assume the old placement; then implement-task T354
- create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta
## log
- 260925 update-ssot done: POST r14 MEM r3 — 분야 and 기억 사용 move from ①'s panel into the writing brief (POST-51✎ POST-54✎ POST-71✎ POST-82✎ MEM-18✎ MEM-26✎), and the brief's run options (목표 분량, 태그 수, quality ticks, 분야, 기억 사용) save together by its 저장 (POST-81✎ POST-89+); no doing task affected, todo T355 T366 T367 T368 T373 T375 assume the old placement
- 260925 update-ssot POST MEM start: 분야 and 기억 사용 move into the writing brief
- 260925 create-task done: T354..T375 (22 todo) from review/published-quality-260924 (29 findings, now converted) and this wave's POST MEM MODEL QUAL GUIDE GEN TMPL THEME-42 deltas; P1 first: T354 (F1 F2, plus the same native-effort drop in A/B write candidates), T355 (F4), T356 (F18, THEME-42)
- 260925 POST r12..r13 POST-77 no-op (no code impact: the shared URL fixture already refuses the blog's home)
- 260924 POST r13: POST-88 removed and POST-20 back to its r11 text — an options save's presence rules are a request contract for the F4 task's impl notes, not product behavior
- 260924 create-task review/published-quality-260924 + POST MEM MODEL QUAL GUIDE GEN TMPL THEME start (this wave's deltas only)
- 260924 update-ssot done: POST r12 (POST-20✎ POST-77✎ POST-79✎ POST-88+ options saves are partial, a taken candidate is spent), MEM r2 MEM-19✎ + MODEL r17 MODEL-30✎ + GEN-18✎ a write comparison freezes 기억, GEN-14✎ GEN-51✎ ticked rules move to the per-post half, GEN-53✎, TMPL r11 TMPL-51✎ the title form binds only a title request in revise, GUIDE r6 GUIDE-40+ QUAL r4 QUAL-41✎ QUAL-46+, THEME r18 THEME-42+; no doing task affected
- 260924 update-ssot POST MEM MODEL QUAL GUIDE GEN TMPL THEME start (review/published-quality-260924 notes)
- 260924 review-code published-quality-260924 ready; 29 findings adopted: 4 P1 (F1 durable generate drops the native-effort flag since 260905, F2 its six hand-copied hops, F4 target length lost or split by mixed presence, F18 InlinePopover Tab), 8 P2, 17 P3; gates green
- 260924 review-code published-quality-260924 start (scope: T322..T353, fe065cdc..f42a3c95)
- 260924 T353 done; ticking or unticking 기억 사용 resends the post's 목표 글자 수 with the flag, so a stored length survives the toggle and its refetch, a natural-length post stays natural, and the tag count stays absent and kept; FE gates pass (p35)
- 260924 finding (T353): a 기억 사용 toggle or a 발행 글 점검 tick pressed while the brief's own length save is still in flight resends the length the post held before it, and the later request wins, so the new number can be lost in that round trip (p35)
- 260924 T353 claimed (p35)
- 260924 T352 done; a durable generate's write prompt carries the memories Start froze into the payload as one [기억] section in the per-post half, in payload order, with the stable half unchanged; the drain never asks the memory port, and a job with no memories (option off or a legacy payload) builds the prompt it built before; BE gate passes (p35)
- 260924 finding (T352): the write-comparison snapshot (`SnapshotWriteInput`) freezes 지침, quality rules and 분야 phrases but no 기억, so an A/B write of a post with 기억 사용 on compares prompts without its memories; MODEL-30 and MEM-19 do not say whether it should (update-ssot MEM MODEL) (p35)
- 260924 T352 claimed (p35)
- 260924 T351 done; ② marks each stored replacement candidate whose source still stands (the title, a tag by index, TEXT without a slot, HEADING, QUOTE, the first LIST item holding it) at its first occurrence, the first in answer order winning an overlap, with at most three phrases and no tag phrase that would empty or duplicate a tag; a take is an ordinary content save that returns 확정 to 검토, drops its mark at render, keeps its neighbours and focuses the pencil that held it; opening or ignoring a mark sends nothing, and a published post or an open editor shows none; FE gates pass (p35)
- 260924 T351 claimed (p35)
- 260924 T350 done; /guidelines opens with the 상위 노출 단어 사용 preset above the list and the empty state: its server text read-only, its own switch and a 적용할 분야 picker that is its fields scope, each saving its own half and showing the answered state at once, a line saying the owner's guidelines win, a prompt while it is on with no 분야, and a refusal in the catalogue's words; FE gates pass (p35)
- 260924 T350 claimed (p35)
