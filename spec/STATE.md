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
| POST | 11 | 11 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 10 | 10 | - | 0 |
| MODEL | 16 | 16 | - | 0 |
| TMPL | 10 | 10 | - | 1 |
| GUIDE | 5 | 5 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 17 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 43 | 40 | CLIP-13✎ | 1 |
| CDS | 25 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 1 | 1 | - | 2 |
| QUAL | 3 | 3 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |
| publishing-260922 | converted@260922 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T330 | Frozen 분야 phrases in per-post material and the write answer's replacement candidates | GEN QUAL POST ARCH | T324 | todo |
| T331 | Quality metrics M1–M4, bands, verdicts and rule texts as pure code | QUAL ARCH | T325 | todo |
| T332 | Rebuild guidelines for the fields scope, and add the 분야 link and preset tables | GUIDE ARCH | T323 | todo |
| T333 | Templates store, validate and render a title area into the frozen brief | TMPL GEN ARCH | T322 T323 T324 T327 | todo |
| T334 | A published post refuses every write except replacing or clearing its URL and deleting it | POST GEN MODEL ARCH | T329 | todo |
| T335 | QualityService: stored per-revision measurements, the read-time aggregate, rule texts and phrase reads | QUAL POST ARCH | T329 T331 | todo |
| T336 | FE templates carry the title area, and ① lists title-area asks first | TMPL ARCH | T333 | todo |
| T337 | A post saves its 분야 through the draft save and its quality ticks as a generation option | POST QUAL ARCH | T334 | todo |
| T338 | The daily per-분야 phrase extraction and refresh pass | QUAL GUIDE ARCH | T335 T326 | todo |
| T339 | 발행됨 is the fourth status everywhere, and a published post reads as locked | POST ARCH THEME | T334 | todo |
| T340 | The template screen authors a title area in the builder and in 원문 | TMPL ARCH THEME | T336 | todo |
| T341 | Write results store nouns and replacement candidates beside the content | GEN POST ARCH | T337 T330 | todo |
| T342 | Guidelines scoped by 분야, the 상위 노출 단어 사용 preset, and 분야-aware freezing | GUIDE GEN ARCH | T332 T337 T330 | todo |
| T343 | ③ records, replaces and clears the post's Naver address | POST ARCH THEME | T339 | todo |
| T344 | Generation freezes the ticked rule texts and the 분야 phrase list at enqueue | GEN POST QUAL GUIDE ARCH | T335 T342 T341 T333 | todo |
| T345 | ① picks the post's 분야 and autosaves it through the draft queue | POST QUAL ARCH THEME | T337 T343 | todo |
| T346 | ② shows this post's own M2, M3 and M4 above the article | POST QUAL ARCH THEME | T335 T343 | todo |
| T347 | /guidelines creates, rescopes and badges guidelines by 분야 | GUIDE ARCH THEME | T342 | todo |
| T348 | The dev seed shows published posts, nouns, candidates, a phrase list and a title-area template without Naver keys | ARCH POST GEN QUAL TMPL | T341 T335 T333 | todo |
| T349 | The writing brief offers one quality row per metric, with a toggletip when over band | POST QUAL ARCH THEME | T328 T344 T345 T346 | todo |
| T350 | /guidelines pins the 상위 노출 단어 사용 preset row with its switch and 적용할 분야 | GUIDE QUAL ARCH THEME | T347 | todo |
| T351 | ② marks replacement spans over the title, tags and body, and taking one is a manual edit | POST GEN QUAL ARCH THEME | T349 | todo |
| T352 | A durable generate carries its frozen memories into the write prompt | MEM GEN ARCH | - | todo |
| T353 | Toggling 기억 사용 keeps the post's 목표 글자 수 | POST MEM ARCH | - | todo |

## next
- implement-task T330 (then the dep order in the tasks table; T330, T331, T332 and T334 are unblocked)
- T352 and T353 are independent pre-existing bug fixes (frozen memories never reach a durable generate; the 기억 사용 toggle clears 목표 글자 수)
- create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta
## log
- 260924 T329 done; a finalized post becomes published from a pasted Naver Blog address (normalized, shared fixture), is replaced or cleared back to finalized, stays learnable, and exposes the published window; only content-writing jobs block the save; BE gate, gen:sql and lint:retirement pass (p35)
- 260924 T329 claimed (p35)
- 260924 T328 done; shared/ui gains InlinePopover and Toggletip on a useAnchoredPanel hook extracted from Listbox, whose behaviour is unchanged, and Popover treats a press in any anchored panel as inside; 2376 FE tests and the FE gates pass (p35)
- 260924 T328 claimed (p35)
- 260924 T327 done; the Go and TS grammars parse a title area (literal, write, ask) and refuse a slot, repeat or note there as not_in_title, one ask namespace and ceiling across both areas, title first; fixture +12 cases and 200 title corpus pairs; BE gate and FE gates pass (p35)
- 260924 T327 claimed (p35)
- 260924 T326 done; internal/naversearch reads one page of the blog search as plain titles and descriptions, and config carries the optional Naver pair and a refresh-interval override; full BE gate and the deploy tests pass (p35)
- 260924 T326 claimed (p35)
- 260924 T325 done; quality gains NFC 어절 tokens, block units that skip slots, containment by language, 8-어절 run matching with its rune share and maximal runs, and sentence splitting; x/text v0.42.0 is now a direct require; full BE gate passes (p35)
- 260924 T325 claimed (p35)
- 260924 T324 done; the write prompts carry the sourced title prohibitions (a title-form variant when the template authored one), the tag rule, the rewritten vocabulary precedence, the ticked-rules and title-area sections, and a bounded nouns answer; both write goldens regenerated, full BE gate passes (p35)
- 260924 T324 claimed (p35)
- 260924 T323 done; migration 0077 adds the six post columns, the published index, post_measurements, field_phrase_lists and templates.title_area, reversible to 76; nullable columns carry no explicit NULL because sqlc reads `TEXT NULL` as untyped; full BE gate and gen:sql pass (p35)
- 260924 T323 claimed (p35)
- 260924 T322 done; the 발행됨/분야/quality/preset/title-area wire exists on both sides with the five new reasons emitted, the 분야 catalogue in `quality` and its closed wire mappers; 2336 FE tests, full BE gate, codegen reproduction and buf lint/breaking pass (p35)
- 260924 T322 claimed (p35)
- 260924 create-task QUAL POST GEN GUIDE TMPL complete; T322..T351 from the handoff plan (QUAL r3 closed the M3/M4 rule-text gap), plus T352 T353 for two pre-existing bugs found while writing them; handoff folder removed
- 260924 update-ssot QUAL complete; r3 decides M3's and M4's rule texts (no repeated noun, cover the title; at least three block types from what the material fits) and makes every rule text yield to natural writing
- 260924 update-ssot QUAL start; the M3 and M4 rule texts carry no decided substance (T331 SSOT-GAP)
- 260923 create-task QUAL POST GEN GUIDE TMPL paused for a machine switch; mapping, SSOT fixes and the 30-task plan are done, task files not yet written; handoff in spec/handoff/create-task-260923/
