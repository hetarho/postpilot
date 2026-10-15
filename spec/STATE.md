# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en
- push: main

## active
| tag | host | mode | since | at | doing |
|---|---|---|---|---|---|
| m7q2 | ihaelam-ui-MacBookPro | autopilot | 2026-10-15T12:20Z | 2026-10-15T12:41Z | T686 |
| p4f1 | ihaelam-ui-MacBookPro | autopilot | 2026-10-15T13:02Z | 2026-10-15T13:02Z | - |

## inbox
- 261015 decided ideation/post-faithful-clip: what does a clip take from a finished post? → the canonical content (title, body, section order, ratings, impressions) as owner material for every writing call (alt: the storyline only | title and memo as today)
- 261015 decided ideation/post-faithful-clip: where is a post linked? → both sides: ③ 이 체험으로 클립 만들기 and the clip ① 글 연결, one post per clip, matching titles first (alt: post side only | clip side only)
- 261015 decided ideation/post-faithful-clip: how are videos placed into post sections? → capture time first, the analysis confirms the content, content matching without a capture time (alt: AI content matching only | free AI composition)
- 261015 decided ideation/post-faithful-clip: form? → keep the CLIP-213 arc, leave out stages the post gives nothing for, the 상호 in the hook and the owner verdict in the outro (alt: a separate 쇼츠 리뷰 style | leave form to T688)
- 261015 decided T696: repair the existing red CI before T686 or defer it? → repair the stale wizard entrypoint test first (alt: defer until T687)
- 261015 decided -: push branch for finished units? → main, and AGENTS.md now defers to haeram-spec-creator with ARCH-31 run before each push and reused on unchanged trees (alt: - commit only | a work branch)
- 261015 decided GUIDE-41: an owner's stored off-state for a rule that became a 원칙 or was split? → ignored for 원칙; 맛은 재료로 살리기 keeps the old impression key and 알려준 사건 순서대로 its own (alt: reset every off-state | keep those rules switchable for owners who turned them off)
- 261015 decided POST-117 CLIP-210: 체험단 글 default? → new posts and clips on, earlier ones off; off hides but keeps the guide (alt: everything on | everything off)
- 261015 decided POST-118 POST-119: advertiser guide shape? → pasted free text plus optional counted items (keywords, min characters, min photos, hashtags); each free-text line is an owner tick (alt: free text parsed by AI | counted items only)
- 261015 decided CLIP-210: guide items on a clip? → keywords and hashtags only; min characters and min photos are post-only (alt: the same items as posts)
- 261015 decided GEN-88: does revision see the advertiser guide? → no, like the memo; generation, storyline jobs and writing tests freeze it (alt: revision too)
- 261015 decided EXPORT-28: disclosure placeholder? → `대가표시_배너_또는_문구_대가표시` on top of the copied body only, never in canonical content (alt: a line inside canonical content | a bracketed [대가 표시] line)
- 261015 decided TMPL-72 CLIP-213: wizard targets and roles? → six targets (맛집, 카페, 뷰티, 숙소, 체험·여가, 제품) with the listed branches and role lists, 아쉬운 점 in every leaf, medical treatment excluded; clips share one seven-stage arc (alt: four lumped groups | AI-generated role lists)
- 261015 decided TMPL-73: heading suggestions? → one explicit action for all checked sections, three per section in the 기본 voice or plain style (alt: generated on entering each section's step | no voice)
- 261015 decided TMPL-64: template from a post? → the wizard with the post's headings as checked custom sections (alt: AI maps the post onto roles)
- 261015 decided VOICE-65: who speaks for the product in the learning chat? → scripted lines from the code-owned catalog, no model call (alt: an AI chat partner per message)
- 261015 decided CLIP-8 CDS-49: 16:9 and 1:1? → new projects 9:16 only; earlier projects keep their ratio and stay renderable (alt: remove the 16:9/1:1 layouts entirely)
- 261015 decided CLIP-209: channel of clips made before channels? → 네이버 클립, movable by the owner (alt: 인스타 클립 | ask per clip)
- 261015 decided CLIP-211 CLIP-212: upload text? → one explicit action once a plan exists, in the 기본 voice, the other channel's text without a re-render, platform limits confirmed at implementation (alt: written automatically after each render | limits fixed in the SSOT now)
- 261015 decided POST-120 CLIP-214: what a clip takes from a post? → title, memo as instruction, 체험단 글, guide and attached videos; photos stay (alt: add photo sources to clips)
- 261015 decided THEME-38: location of the clip templates and 영상 지침 both clip channels share? → the clip channel they were opened from, direct links under 네이버 클립 (alt: a separate 클립 설정 destination)
- 261015 decided THEME-65: 인스타 게시물 until decided? → no navigation entry, open [?] (alt: a 준비 중 entry)
- 261015 decided MKT-17: positioning line? → 체험단 자동화 with 내가 겪은 그대로, 체험단 글을 빠르게 (alt: keep AI는 속도를, 글의 주도권은 나에게)
- 261015 decided -: keep English output in a Korean 체험단 product? → kept unchanged (alt: Korean only)
- 261015 decided POST-120 T692: a memo longer than the clip instruction bound (1000)? → carry the first 1000 characters and say so in the dialog; the post keeps the full memo (alt: refuse the carry-over | carry no memo)
- 261015 decided THEME-62 T681: colour of the 광고주 요청 evidence role? → the red ramp (300 on 950 dark, 700 on 50 light, contrast 9.98 and 7.80) with a double underline and its label, since THEME-15 closes the palette and red is the only ramp outside the other origins, the accent and the neutral (alt: a new palette ramp, needs THEME-15 and THEME-35 changed | violet, the action accent)
- 261015 suggest T677 T678 T679 T680 T681: review-code over the five campaign tasks done so far (principle tier, advertiser guide storage and editor fields, advertiser origin)
- 261015 decided POST-119 T683: which count meets 최소 글자 수? → the count without spaces, the stricter of the two the checklist shows, so a met mark holds whichever way the campaign counts (alt: with spaces | either one)
- 261015 decided TMPL-72 T685: how does a product kind adapt role wording? → each kind adds its own section after 주요 기능 (가전 설치·소음, 화장품 제형·발림성, 식품 맛·조리법, 육아·반려 안전·반응; 생활·주방 none) (alt: rename the shared roles per kind | no kind-specific wording)
- 261015 decided TMPL-74 T685: what does 직접 입력 place? → the author's typed answer verbatim (a 데이터 받기 field of the fixed-text flavor), and such a section never repeats because the grammar refuses a field inside a repeat (alt: the AI writes the section from the answer)

## ideation
| id | st |
|---|---|
| clip-source-observation-visibility | converted@260912 |
| clip-template-as-preset | converted@260917 |
| post-quality-and-related-links | converted@260923 |
| searchable-details | open@261007 |
| prompt-engineering | converted@261007 |
| creation-and-comparison-ux | open@261007 |
| storyline-first | converted@260927 |
| voice-tidy | converted@260929 |
| daily-credit-plans | converted@260929 |
| template-from-request | converted@261001 |
| familiar-video-editing-and-dubbing | converted@261004 |
| friendly-model-names | open@261013 |
| campaign-reviewer-focus | converted@261015 |
| post-faithful-clip | converted@261015 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 20 | 20 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 38 | 38 | - | 0 |
| POST | 42 | 41 | POST-120✎ | 0 |
| VOICE | 17 | 17 | - | 0 |
| GEN | 31 | 31 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 28 | 28 | - | 0 |
| GUIDE | 23 | 22 | GUIDE-42✎ GUIDE-52✎ | 0 |
| EXPORT | 12 | 12 | - | 0 |
| LANG | 10 | 10 | - | 0 |
| THEME | 34 | 34 | - | 2 |
| MKT | 11 | 11 | - | 0 |
| VIDEO | 8 | 8 | - | 0 |
| CLIP | 61 | 60 | CLIP-215+ CLIP-216+ CLIP-217+ CLIP-218+ CLIP-1✎ CLIP-69✎ CLIP-141✎ CLIP-178✎ CLIP-211✎ CLIP-214✎ | 2 |
| CDS | 34 | 34 | - | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 7 | 7 | - | 2 |
| QUAL | 8 | 8 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| EDIT | 6 | 6 | - | 0 |
| INFRA | 2 | 0 | all | 1 |

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
| clip-narrate-failure-260926 | converted@260926 |
| conformance-all-260927 | converted@260927 |
| perf-cost-261004 | converted@261005 |
| desktop-ux-policy-261007 | converted@261007 |
| all-261008 | converted@261008 |
| structure-261008 | converted@261008 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |
| T686 | Suggest section headings in the owner's voice | TMPL EDIT | T685 | doing@261015.m7q2 |
| T687 | Make the wizard the only way to create a post template | TMPL EDIT | T685 T686 | todo |
| T688 | Build the video-template wizard and make it the only way to create one | CLIP TMPL EDIT | T685 T686 | todo |
| T689 | Give clips a channel and create new clips vertical only | CLIP CDS | - | todo |
| T690 | Add the 체험단 글 switch and advertiser guide to clips | CLIP GEN GUIDE | T679 T680 T689 | todo |
| T691 | Write each clip channel's upload text on request | CLIP EXPORT | T689 T690 | todo |
| T692 | Create a clip from a finished post | POST CLIP | T680 T689 T690 | todo |
| T693 | Learn a voice through a scripted chat and grow AI styles toward it | VOICE THEME | - | todo |
| T694 | Navigate by channel with 내 설정 beside it | THEME POST CLIP GUIDE TMPL VOICE | T689 | todo |
| T695 | Tell the 체험단 story on the public about page | MKT | T682 T683 T684 T687 T688 T691 T692 T693 T694 | todo |
| T697 | Expose Content-Range on private storage and gate the deploy on it | ARCH INFRA | - | todo |

## next
- create-task CLIP POST GUIDE (p4f1): the linked-post clip delta; refresh the bases of T689–T692.
- implement-task T697 (production clip generation is down: the live R2 CORS hides Content-Range; the owner applies deploy/r2-cors.json first).
- Autopilot m7q2: implement T686–T695 in dependency order; T696 repaired the existing red CI entry regression. Previous provisional decisions remain in inbox.

## log
- 261015 update-ssot CLIP r61 POST r42 GUIDE r23 done: a linked post as the clip's story (ideation/post-faithful-clip); no doing task affected (T686 is TMPL).
- 261015 update-ssot CLIP POST GUIDE start (ideation/post-faithful-clip)
- 261015 start-vibe p4f1 start (autopilot): ideation post-faithful-clip → update-ssot → create-task, overlap check with the template tasks; no implementation (m7q2 implements).
- 261015 T686 start on main at 13f778bb (m7q2 autopilot); T696 pushed at 832c357e with CI38736338662 running.
- 261015 ideation post-faithful-clip ready (autonomous at the owner's request; decisions in inbox)
- 261015 T696 done; full local CI passes on the repaired template wizard entry regression.
- 261015 ideation post-faithful-clip start
- 261015 create-task T697 done: production clips fail with CLIP_SOURCE_RANGE_INVALID because the live bucket exposes only ETag; probe-backed deploy gate plus HeadSource cause/cancellation.
- 261015 create-task bug/clip-source-range start
- 261015 T696 start on main at 0c4c997b; initial push waits for repair of the diagnosed existing CI failure.
- 261015 create-task T696 done: current wizard entrypoint regression for failed CI 38708735705; autopilot m7q2 selected by the owner.
- 261015 start-vibe m7q2 start on main at 0c4c997b; awaiting mode and route, no other active session.
- 261015 autopilot v7k2 stopped at the owner's request after T685; report spec/reports/261015-v7k2.md
- 261015 T685 done
- 261015 T685 start on main at ce4e027c.
- 261015 T684 done
- 261015 T684 start on main at d64bd979.
- 261015 T683 done
- 261015 T683 start on main at 46e4c976.
- 261015 T682 done
- 261015 T682 start on main at 4100f145.
- 261015 T681 done
- 261015 T681 start on main at d52654b1.
