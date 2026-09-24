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
| POST | 12 | 11 | POST-20✎ POST-77✎ POST-79✎ POST-88+ | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 11 | 10 | GEN-14✎ GEN-18✎ GEN-51✎ GEN-53✎ | 0 |
| MODEL | 17 | 16 | MODEL-30✎ | 0 |
| TMPL | 11 | 10 | TMPL-51✎ | 1 |
| GUIDE | 6 | 5 | GUIDE-40+ | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 18 | 15 | THEME-19✎ THEME-42+ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 43 | 40 | CLIP-13✎ | 1 |
| CDS | 25 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 2 | 1 | MEM-19✎ | 2 |
| QUAL | 4 | 3 | QUAL-41✎ QUAL-46+ | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |
| publishing-260922 | converted@260922 |
| published-quality-260924 | ready@260924 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|

## next
- create-task review/published-quality-260924 + POST MEM MODEL QUAL GUIDE GEN TMPL THEME (this wave's deltas only; P1 F1 F2 F4 F18 first)
- create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta
## log
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
- 260924 T349 done; ①'s brief shows 발행 글 점검 after 목표 분량 with M1–M4 in their four states from the account aggregate: an over-band row is a checkbox whose toggletip quotes the rule text it adds, ticks autosave the whole set with the length, a stored within-band tick is kept, the boxes hold still for a running job or the publish lock, and the read is prefetched, follows the target language and refreshes after URL, content and delete saves; FE gates pass (p35)
- 260924 T349 claimed (p35)
- 260924 T348 done; `pnpm dev --seed` shows the quality surfaces with no Naver keys: pro and master publish 2 and 11 posts at valid Naver addresses (master meeting every minimum), every generated post stores nouns its text contains, pro's and master's posts carry 일상·생각 with three candidates on each review post, master's first draft uses the 하루 기록 title-area template, and the daily_life phrase list is written only when absent and survives every seed; BE gate passes (p35)
- 260924 T348 claimed (p35)
- 260924 T347 done; the shared scope control offers 전역, 특정 템플릿 and 특정 분야 with the nine 분야 as a catalogue-order checkbox list (새 지침, 승인 and the whole-scope edit), each kind clearing the other's set; a 분야 create or rescope sends kind and both sets in one shape, a 분야 guideline badges one chip per 분야, the list keeps the server's three groups, a refusal keeps the draft, and an unreadable scope fails the read; FE gates pass (p35)
- 260924 T347 claimed (p35)
