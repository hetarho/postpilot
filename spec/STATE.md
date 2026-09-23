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
| T323 | One additive migration for publication, 분야, nouns, candidates, ticks, measurements, phrase lists and the title area | POST QUAL GEN TMPL ARCH | - | todo |
| T324 | The write prompt gains the title and tag rules, the vocabulary precedence, the quality and title-area sections, and a nouns answer | GEN GUIDE TMPL QUAL ARCH | - | todo |
| T325 | Quality text primitives: Korean and English containment, 8-어절 run hashing and sentence splitting | QUAL ARCH | - | todo |
| T326 | The 네이버 검색 API blog client and its optional credentials | QUAL ARCH | - | todo |
| T327 | The template grammar parses a title area in lockstep in Go and TypeScript | TMPL ARCH | - | todo |
| T328 | shared/ui InlinePopover and Toggletip on one extracted anchored-panel hook | POST THEME ARCH | - | todo |
| T329 | A finalized post becomes 발행됨 from a pasted Naver URL, replaceable and clearable | POST QUAL ARCH | T322 T323 | todo |
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
- implement-task T323 (then the dep order in the tasks table; T323..T328 have no dep and can run in parallel sessions)
- T352 and T353 are independent pre-existing bug fixes (frozen memories never reach a durable generate; the 기억 사용 toggle clears 목표 글자 수)
- create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta
## log
- 260924 T322 done; the 발행됨/분야/quality/preset/title-area wire exists on both sides with the five new reasons emitted, the 분야 catalogue in `quality` and its closed wire mappers; 2336 FE tests, full BE gate, codegen reproduction and buf lint/breaking pass (p35)
- 260924 T322 claimed (p35)
- 260924 create-task QUAL POST GEN GUIDE TMPL complete; T322..T351 from the handoff plan (QUAL r3 closed the M3/M4 rule-text gap), plus T352 T353 for two pre-existing bugs found while writing them; handoff folder removed
- 260924 update-ssot QUAL complete; r3 decides M3's and M4's rule texts (no repeated noun, cover the title; at least three block types from what the material fits) and makes every rule text yield to natural writing
- 260924 update-ssot QUAL start; the M3 and M4 rule texts carry no decided substance (T331 SSOT-GAP)
- 260923 create-task QUAL POST GEN GUIDE TMPL paused for a machine switch; mapping, SSOT fixes and the 30-task plan are done, task files not yet written; handoff in spec/handoff/create-task-260923/
- 260923 update-ssot complete; QUAL r2 POST r11 GEN r10 GUIDE r5 TMPL r10 close 34 of the 42 mapping gaps (a published post refuses every write but URL replace/clear and delete; the write pass returns nouns; the ② offer needs only a 분야; revise drops the preset line); the other 8 are implementation rules for the tasks
- 260923 create-task QUAL POST GEN GUIDE TMPL start; QUAL r1 all, POST r10, GEN r9, GUIDE r4, TMPL r9
- 260923 the five domains' opens all closed ahead of create-task; measurement split into a per-post layer shown from ② and an aggregate over 발행됨 posts only, vocabulary ranked guideline > template > voice with the preset injected last, and 분야 picked on ①'s panel
- 260923 ideation post-quality-and-related-links converted; QUAL r1 created and POST r10, GEN r9, GUIDE r4, TMPL r9 carry the rest; 추천글 and GEO stay parked in the ideation doc
- 260923 update-ssot TMPL complete; r9 adds the optional title area limited to literal text, `<write>` and `<ask>`
- 260923 update-ssot GUIDE complete; r4 adds the preset, the `fields` scope kind and the precedence narrowing, with `templatePrecedence`'s identical wording left as [?]
- 260923 update-ssot GEN complete; r9 adds the frozen 분야 phrase list, the write answer's replacement candidates, the two sourced title prohibitions, the tag rule and the ticked quality rules' position
- 260923 update-ssot POST complete; r10 adds the 발행됨 status and its pasted URL, the content lock that spares export/copy/말투 학습, the fourth badge and filter, ②'s replacement spans and the brief's quality checkboxes
- 260923 create-ssot QUAL complete; r1 carries four metrics over 발행됨 posts with per-metric minimums, stored per-post measurements, and the daily 네이버 검색 API phrase batch; bands, 분야 names and English-target analysis stay [?]
- 260923 ideation post-quality-and-related-links ready; 10 rounds, v1 is the 발행됨 status and its measurement set, four metrics with offered rules, an optional template title area, and per-분야 phrases reaching a post by two routes; 추천글 and GEO stay parked
- 260923 ideation post-quality round 10; Jev examined and rejected for v1 against the owner's own test (price not lower — the write pass adds no call while Jev resends the post per span; quality unevidenced), so the write pass returns candidate spans as JSON; a chosen replacement is a manual edit; the 발행됨 lock covers content edits only
- 260923 ideation post-quality round 9; the 지침 and ②'s offer split by what the source contains (same meaning already stated → substituted in the prompt; absent → offered on hover), so rounds 5-6 stand; candidate spans come from the write pass as JSON; the offer covers body, title and tags; clearing the URL unlocks a 발행됨 post back to 확정
- 260923 ideation post-quality round 7-8; the doc's PUB-15/22/25 footing was retired on 260922, so a manual Naver URL now enters a new terminal 발행됨 status that supplies both the measured title set and the future link pool; body vocabulary leaves the write prompt entirely and becomes a hover replacement offer on ②; every rule is language-blind
- 260923 ideation post-quality round 5-6; 분야 becomes a per-post attribute and a third 지침 scope kind; one fixed 상위 노출 단어 사용 preset with a multi-select 적용할 분야 keeps GUIDE-18 intact; guidelinePrecedence narrows 어휘 out; 11 of 17 opens are parked with 추천글/GEO
