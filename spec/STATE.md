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
| m7q2 | ihaelam-ui-MacBookPro | autopilot | 2026-10-15T12:20Z | 2026-10-15T17:48Z | - |

## inbox
- 261016 suggest T696 T686 T687 T688 T689: review-code over the completed template-wizard, heading-job and clip-channel units
- 261015 decided T698: how is a deleted or reused post slug told apart? → store the post's creation time beside the slug and resolve at read; no foreign key (alt: FK ON DELETE SET NULL, which nulls user_id on SQLite composites | clear links from the post context)
- 261015 decided T699: which part of the post reaches the writer? → title plus headed sections of TEXT/QUOTE/LIST; photo captions, tags and summary left out; an oversized request is refused, never trimmed (alt: the whole content JSON | a fixed character cap)
- 261015 decided T700: where does capture time come from? → JPEG EXIF DateTimeOriginal and MP4/MOV mvhd, parsed in the browser without a new dependency (alt: File.lastModified | an EXIF library)
- 261015 decided T701: how does capture order pick a section? → the sections of the nearest photo before and after, scenes choose between them, file sequence when no capture time (alt: predecessor only | scenes only)
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
| POST | 42 | 42 | - | 0 |
| VOICE | 17 | 17 | - | 0 |
| GEN | 31 | 31 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 28 | 28 | - | 0 |
| GUIDE | 23 | 23 | - | 0 |
| EXPORT | 12 | 12 | - | 0 |
| LANG | 10 | 10 | - | 0 |
| THEME | 34 | 34 | - | 2 |
| MKT | 11 | 11 | - | 0 |
| VIDEO | 8 | 8 | - | 0 |
| CLIP | 63 | 63 | - | 2 |
| CDS | 36 | 36 | - | 1 |
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
| T690 | Add the 체험단 글 switch and advertiser guide to clips | CLIP GEN GUIDE | T679 T680 T689 | todo |
| T691 | Write each clip channel's upload text on request | CLIP EXPORT | T689 T690 | todo |
| T692 | Create a clip from a finished post | POST CLIP | T680 T689 T690 | todo |
| T693 | Learn a voice through a scripted chat and grow AI styles toward it | VOICE THEME | - | todo |
| T694 | Navigate by channel with 내 설정 beside it | THEME POST CLIP GUIDE TMPL VOICE | T689 | todo |
| T695 | Tell the 체험단 story on the public about page | MKT | T682 T683 T684 T687 T688 T691 T692 T693 T694 | todo |
| T697 | Expose Content-Range on private storage and gate the deploy on it | ARCH INFRA | - | todo |
| T698 | Link one finished post to a clip from either side | CLIP POST | T689 T690 T692 T694 | todo |
| T699 | Send the linked post's text to every clip writing call | CLIP GUIDE QUOTA | T698 T691 | todo |
| T700 | Record when post photos, post videos and clip sources were shot | CLIP | T692 | todo |
| T701 | Build the storyline in the linked post's order and place videos by capture order | CLIP GUIDE | T699 T700 | todo |

## next
- After T686–T695 (m7q2): implement-task T698 → T699, T700 → T701 (a linked post as the clip's story); T697's operator CORS prerequisite is complete.
- implement-task T697 (the owner applied production Content-Range CORS261015; add the ranged-GET probe/deployment gate and preserve HEAD cancellation).
- Autopilot m7q2: T689 implementation/full local CI complete; finish image/media gate and push, then T690–T695 and T698–T701. T697 is ready after the owner CORS change.

## log
- 261016 T689 done; full Go and FE549/4359 pass; final channel-only writes preserve completed content and original retention; image/media push gate follows.
- 261016 T688 remote CI38774298150, Deploy38774298135 and Media38774966073 all succeed for2ce237dd; T689 claimca5c024c CI38774348751 succeeds.
- 261016 T689 start on main at2ce237dd (m7q2 autopilot); CLIP r62/r63 and CDS r35/r36 source-colour changes are unrelated; bases refreshed to63/36.
- 261016 T688 pre-push image/media gates pass atb7a51f49; source7462310f unchanged after spec-only records.
- 261016 T688 done; whole FE549 files/4352 tests and full BE/CI checks pass; deployment/media push gate follows.
- 261016 T688 freshness: CLIP r62/r63 HDR-source changes do not affect the wizard; base refreshed to63. Concurrent source-color commits remain intact.
- 261015 HDR scope cut at the owner's word ("에러만 안 나면"): CLIP r63 HLG used as is, PQ refused; CDS r36 reverts the colour tags; T702–T704 removed; HLG accepted in source-color.ts directly, no task
- 261015 autopilot h7c3 ended: CLIP r62 CDS r35, T702–T704; report spec/reports/261015-h7c3.md
- 261015 create-task CLIP r62 CDS r35 done: T702 (explore 3h) → T703; T704 independent
- 261015 update-ssot CLIP r62 CDS r35 done: HDR sources through one versioned SDR conversion; output tagged BT.709; no doing task affected (T686 is TMPL)
- 261015 autopilot h7c3 start: HDR source support (CLIP_SOURCE_COLOR_UNSUPPORTED on HLG phone footage) → update-ssot CLIP CDS → create-task; no implementation
- 261015 update-ssot CLIP CDS start (HDR sources)
- 261015 T688 start on main atc2f3c631 (m7q2 autopilot); T687 final CI passed locally, remote CI38753876172 pending.
- 261015 T687 final full CI passes at2cc2ed05 (FE544 files/4334 tests); unchanged backend/schema/deployment inputs reuse T686 evidence.
- 261015 T687 done; selected116 tests, lint/FSD and build pass; full pre-push FE checks follow.
- 261015 T687 start on main atc3de0b08 (m7q2 autopilot); T686 local CI/deployment/media pass, pushed with remote workflows pending.
- 261015 T686 pre-push deployment/media gates pass at73458eb9; source unchanged after spec-only verification records.
- 261015 T686 done; full FE/BE checks pass, deployment/media push gate follows.
- 261015 autopilot p4f1 ended: ideation/post-faithful-clip converted to CLIP r61 POST r42 GUIDE r23 and T698–T701; report spec/reports/261015-p4f1.md
- 261015 create-task CLIP r61 POST r42 GUIDE r23 done: T698–T701; T688 T689 T690 T691 T692 T694 bases refreshed, no scope change; T686 T687 (TMPL) unaffected.
