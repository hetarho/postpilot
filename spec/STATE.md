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
| T347 | /guidelines creates, rescopes and badges guidelines by 분야 | GUIDE ARCH THEME | T342 | todo |
| T348 | The dev seed shows published posts, nouns, candidates, a phrase list and a title-area template without Naver keys | ARCH POST GEN QUAL TMPL | T341 T335 T333 | todo |
| T349 | The writing brief offers one quality row per metric, with a toggletip when over band | POST QUAL ARCH THEME | T328 T344 T345 T346 | todo |
| T350 | /guidelines pins the 상위 노출 단어 사용 preset row with its switch and 적용할 분야 | GUIDE QUAL ARCH THEME | T347 | todo |
| T351 | ② marks replacement spans over the title, tags and body, and taking one is a manual edit | POST GEN QUAL ARCH THEME | T349 | todo |
| T352 | A durable generate carries its frozen memories into the write prompt | MEM GEN ARCH | - | todo |
| T353 | Toggling 기억 사용 keeps the post's 목표 글자 수 | POST MEM ARCH | - | todo |

## next
- implement-task T347 (then the dep order in the tasks table; T347, T348 and T349 are unblocked)
- T352 and T353 are independent pre-existing bug fixes (frozen memories never reach a durable generate; the 기억 사용 toggle clears 목표 글자 수)
- create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta
## log
- 260924 T346 done; ② shows one 이 글의 측정값 row directly above the article on a post with content that is not published: M2, M3 and M4 in three groups with the server's band edges, 주의 or 양호 only where a band was judged, 측정할 수 없어요 for an unset value and 0 for a stored one, M2's minimum line while under it, the bands-are-ours line, and loading and failure lines; it reads per content revision and refetches after content, URL and delete saves; FE gates pass (p35)
- 260924 T346 claimed (p35)
- 260924 T345 done; ① picks the post's 분야 under the data fields and above 기억 사용 (before the photos on /posts/new) through a fifth draft-queue channel with the 템플릿's presence rules: a create carries a chosen 분야 and omits 없음, a saved post sends a pick at once and 없음 as a present clear, a refusal is taken back with its reason under the field, and a published post shows it disabled under T339's one reason; FE gates pass (p35)
- 260924 T345 claimed (p35)
- 260924 T344 done; StartGeneration and the write-comparison snapshot freeze the ticked rule texts still over band (in the run's language) and the 분야's first 30 phrases, the job row and the worker carry both, the drain and a revision never ask the quality context, and a post with nothing ticked and no list writes the payload and prompt it wrote before; end to end the phrase section and the preset line reach the write and never the revision; BE gate passes (p35)
- 260924 T344 claimed (p35)
- 260924 T343 done; ③ ends with a 발행 field that pastes, replaces or clears the post's Naver address through SavePostPublishedUrl, refuses a non-Naver address in place with the server's own sentence before sending (the shared fixture pins the pre-check), stays closed with its reason before 확정, and shows server refusals under the field; FE gates and lint:retirement pass (p35)
- 260924 T343 claimed (p35)
- 260924 T342 done; a guideline can be scoped to listed 분야 (all three shapes validated, collapsed, rescoped as one normalized patch), the owner switches the 상위 노출 단어 사용 preset and picks its 분야 as a presence patch outside the cap and text uniqueness, and every generation and write snapshot freezes global, template and 분야 texts with the preset line last while a revision never carries it; only the owner's procedures write guideline rows (structural test); BE gate passes (p35)
- 260924 T342 claimed (p35)
- 260924 T341 done; a generation and an applied write winner replace the post's stored nouns and replacement candidates (none clears them), a revision and a manual save keep them, an identical content with other annotations is a new machine write, Post.replacement_candidates carries them in stored order while nouns stay off the wire, and a lab candidate's output carries both with a legacy output reading as none; BE gate and gen:sql pass (p35)
- 260924 T341 claimed (p35)
- 260924 T340 done; the template screen authors an optional 제목 형식 above 템플릿 구성 under one 블록 · 원문 switch: the title builder offers only AI가 쓰는 글 and 고정 문구 on one line joined by spaces, a body row asking under a title label says so and stays out until the title lets it go, 제목 원문 has its own field, counter and failure, an unreadable title is fixed or cleared from its own section, and the parse refusal keeps its area; FE gates pass (p35)
- 260924 T340 claimed (p35)
- 260924 T339 done; the FE knows published everywhere the status union is exhaustive (label, steps, filter, URL, badge) and a published post reads as locked: it opens on ③, ② reads its prose under one sentence with only the road onward, ① is read-only with that sentence once and sends no write, ③ keeps learning and export, and a draft or content save refused as published is an answer that refetches instead of retrying; FE gates pass (p35)
- 260924 T339 claimed (p35)
- 260924 T338 done; an in-process pass keeps each 분야's top-50 phrase list fresh from up to three pages of Naver blog results through the one extractor (2-5 tokens, never only stopwords, counted once per title or description, subsumed, ranked), on a durable per-field next refresh with a boot catch-up off the listener path and a retry that keeps the last list; without both Naver keys nothing runs; BE gate passes (p35)
- 260924 T338 claimed (p35)
- 260924 T337 done; a post saves its 분야 through SavePostDraft (presence-aware, on create in the same insert, validated through a consumer-owned field directory) and its quality ticks through SavePostGenerationOptions (canonical order, cleared by an empty set), neither touching the lifecycle; SaveDraft takes a DraftSave; PostInput carries both as json:"-" inputs; BE gate and gen:sql pass (p35)
- 260924 T337 claimed (p35)
