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
| v7k2 | DESKTOP-VKPLS8P | autopilot | 2026-10-15T03:06Z | 2026-10-15T08:07Z | - |

## inbox
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

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 20 | 20 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 38 | 38 | - | 0 |
| POST | 41 | 41 | - | 0 |
| VOICE | 17 | 17 | - | 0 |
| GEN | 31 | 31 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 28 | 28 | - | 0 |
| GUIDE | 22 | 22 | - | 0 |
| EXPORT | 12 | 12 | - | 0 |
| LANG | 10 | 10 | - | 0 |
| THEME | 34 | 34 | - | 2 |
| MKT | 11 | 11 | - | 0 |
| VIDEO | 8 | 8 | - | 0 |
| CLIP | 60 | 60 | - | 2 |
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
| T684 | Keep the Naver output alone and reserve the disclosure place | EXPORT VIDEO LANG POST MKT | T679 | todo |
| T685 | Build the guided post-template wizard | TMPL THEME | - | todo |
| T686 | Suggest section headings in the owner's voice | TMPL EDIT | T685 | todo |
| T687 | Make the wizard the only way to create a post template | TMPL EDIT | T685 T686 | todo |
| T688 | Build the video-template wizard and make it the only way to create one | CLIP TMPL EDIT | T685 T686 | todo |
| T689 | Give clips a channel and create new clips vertical only | CLIP CDS | - | todo |
| T690 | Add the 체험단 글 switch and advertiser guide to clips | CLIP GEN GUIDE | T679 T680 T689 | todo |
| T691 | Write each clip channel's upload text on request | CLIP EXPORT | T689 T690 | todo |
| T692 | Create a clip from a finished post | POST CLIP | T680 T689 T690 | todo |
| T693 | Learn a voice through a scripted chat and grow AI styles toward it | VOICE THEME | - | todo |
| T694 | Navigate by channel with 내 설정 beside it | THEME POST CLIP GUIDE TMPL VOICE | T689 | todo |
| T695 | Tell the 체험단 story on the public about page | MKT | T682 T683 T684 T687 T688 T691 T692 T693 T694 | todo |

## next
- implement-task T684 (then dep order … T695); 21 provisional decisions wait in inbox for the owner (they will read the report first).
- ideation friendly-model-names 계속: decide the automatic alias word list (theme, size, contents) and whether speech profiles join; then create-ssot/update-ssot MODEL. Also pending: the 17 unadopted choices in review/structure-261008 and doc-review ARCH.
- push: 65c79f2b passed its pre-push gate and waits only for the SSH key (ssh-add) with the spec commits after it; gh login is needed to read CI. 6e5039c8 is the live green revision.

## log
- 261015 T683 done
- 261015 T683 start on main at 46e4c976.
- 261015 T682 done
- 261015 T682 start on main at 4100f145.
- 261015 T681 done
- 261015 T681 start on main at d52654b1.
- 261015 T680 done
- 261015 T680 start on main at 8db352ff.
- 261015 T679 done on main: posts store the 체험단 글 switch, advertiser guide and checklist ticks (migration 0159, internal/campaign).
- 261015 T679 start on main at a751f4ae.
- 261015 T678 done on main: Postpilot 원칙 card above each guideline list, sheet without 원칙.
- 261015 T678 start on main at 6ff7a0ed.
- 261015 T677 done on main: 원칙 tier in the registry, prompts and wire; local Node default moved to v26.12.0 through fnm at the owner's request (pnpm dev verified).
- 261015 T677 start on main at d6aa385a (v7k2 autopilot).
- 261015 create-task done: T677–T695 from GUIDE r22 GEN r31 POST r41 EXPORT r12 TMPL r28 CLIP r60 VOICE r17 THEME r34 MKT r11 CDS r34 EDIT r6 VIDEO r8 LANG r10; migrations reserved 0159 (T679) 0160 (T689) 0161 (T690) 0162 (T691).
- 261015 create-task GUIDE GEN POST EXPORT TMPL CLIP VOICE THEME MKT CDS EDIT VIDEO LANG start (autopilot v7k2).
- 261015 update-ssot GUIDE r22 GEN r31 POST r41 EXPORT r12 TMPL r28 CLIP r60 VOICE r17 THEME r34 MKT r11 CDS r34 EDIT r6 VIDEO r8 LANG r10 done: ideation/campaign-reviewer-focus converted (원칙 tier, advertiser guide + checklist + 광고주 요청 origin, 체험단 글 + disclosure placeholder, template wizard, learning chat, channel navigation, clip channels + upload text, 9:16 for new clips, Naver-only post output); 18 provisional decisions in inbox; no doing task in the blast radius; next create-task.
- 261015 update-ssot GUIDE GEN POST EXPORT TMPL CLIP VOICE THEME MKT start: from ideation/campaign-reviewer-focus (autopilot v7k2).
- 261015 start-vibe v7k2 autopilot start: update-ssot from ideation/campaign-reviewer-focus first; cfg push main.
- 261015 ideation campaign-reviewer-focus ready: always-on principle groups (facts, impressions + constructive honesty, no AI tells, titles/tags), advertiser guide with checklist and a 광고주 요청 origin color, 체험단 글 switch with a top disclosure placeholder, branching template wizard with owner headings, chat voice learning, four channel menus + 내 설정, clip upload text, 9:16 only, Tistory/site/Markdown out.
