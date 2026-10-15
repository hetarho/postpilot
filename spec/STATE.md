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
| v7k2 | DESKTOP-VKPLS8P | autopilot | 2026-10-15T03:06Z | 2026-10-15T03:06Z | - |

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
- 261015 decided LANG: keep English output in a Korean 체험단 product? → kept unchanged (alt: Korean only)

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
| POST | 41 | 40 | POST-117+ POST-118+ POST-119+ POST-120+ POST-112✎ POST-114✎ POST-43✎ POST-54✎ POST-62✎ POST-64✎ POST-107✎ | 0 |
| VOICE | 17 | 16 | VOICE-76+ VOICE-65✎ VOICE-60✎ VOICE-66✎ VOICE-71✎ VOICE-4✎ VOICE-32✎ VOICE-47✎ VOICE-52✎ VOICE-67✎ | 0 |
| GEN | 31 | 30 | GEN-87+ GEN-88+ GEN-80✎ GEN-16✎ GEN-17✎ GEN-44✎ GEN-47✎ GEN-49✎ GEN-50✎ GEN-75✎ GEN-83✎ GEN-15✎ GEN-5✎ GEN-14✎ GEN-30✎ GEN-40✎ | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 28 | 27 | TMPL-71+ TMPL-72+ TMPL-73+ TMPL-74+ TMPL-16✎ TMPL-24✎ TMPL-26✎ TMPL-34✎ TMPL-52✎ TMPL-58✎ TMPL-64✎ | 0 |
| GUIDE | 22 | 21 | GUIDE-51+ GUIDE-52+ GUIDE-53+ GUIDE-54+ GUIDE-16✎ GUIDE-41✎ GUIDE-42✎ GUIDE-1✎ GUIDE-14✎ GUIDE-15✎ GUIDE-17✎ GUIDE-18✎ GUIDE-19✎ GUIDE-20✎ GUIDE-26✎ GUIDE-37✎ GUIDE-43✎ GUIDE-44✎ GUIDE-48✎ | 0 |
| EXPORT | 12 | 11 | EXPORT-6- EXPORT-7- EXPORT-8- EXPORT-28+ EXPORT-1✎ EXPORT-2✎ EXPORT-3✎ EXPORT-9✎ EXPORT-10✎ EXPORT-11✎ EXPORT-12✎ EXPORT-20✎ EXPORT-21✎ EXPORT-22✎ EXPORT-23✎ EXPORT-24✎ EXPORT-25✎ EXPORT-26✎ | 0 |
| LANG | 10 | 9 | LANG-25✎ | 0 |
| THEME | 34 | 33 | THEME-65+ THEME-66+ THEME-38✎ THEME-47✎ THEME-26✎ THEME-62✎ | 2 |
| MKT | 11 | 10 | MKT-17✎ MKT-11✎ | 0 |
| VIDEO | 8 | 7 | VIDEO-14✎ VIDEO-15✎ | 0 |
| CLIP | 60 | 59 | CLIP-209+ CLIP-210+ CLIP-211+ CLIP-212+ CLIP-213+ CLIP-214+ CLIP-8✎ CLIP-130✎ CLIP-1✎ CLIP-3✎ CLIP-11✎ CLIP-27✎ CLIP-41✎ CLIP-42✎ CLIP-183✎ CLIP-184✎ CLIP-208✎ | 2 |
| CDS | 34 | 33 | CDS-49✎ | 1 |
| BILL | 9 | 9 | - | 0 |
| MEM | 7 | 7 | - | 2 |
| QUAL | 8 | 8 | - | 0 |
| GIFT | 3 | 3 | - | 0 |
| DUB | 3 | 3 | - | 0 |
| EDIT | 6 | 5 | EDIT-2✎ EDIT-3✎ EDIT-17✎ | 0 |
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

## next
- create-task GUIDE GEN POST EXPORT TMPL CLIP VOICE THEME MKT CDS EDIT VIDEO LANG (from campaign-reviewer-focus); 18 provisional decisions wait in inbox for the owner.
- ideation friendly-model-names 계속: decide the automatic alias word list (theme, size, contents) and whether speech profiles join; then create-ssot/update-ssot MODEL. Also pending: the 17 unadopted choices in review/structure-261008 and doc-review ARCH.
- push: 65c79f2b passed its pre-push gate and waits only for the SSH key (ssh-add) with the spec commits after it; gh login is needed to read CI. 6e5039c8 is the live green revision.

## log
- 261015 update-ssot GUIDE r22 GEN r31 POST r41 EXPORT r12 TMPL r28 CLIP r60 VOICE r17 THEME r34 MKT r11 CDS r34 EDIT r6 VIDEO r8 LANG r10 done: ideation/campaign-reviewer-focus converted (원칙 tier, advertiser guide + checklist + 광고주 요청 origin, 체험단 글 + disclosure placeholder, template wizard, learning chat, channel navigation, clip channels + upload text, 9:16 for new clips, Naver-only post output); 18 provisional decisions in inbox; no doing task in the blast radius; next create-task.
- 261015 update-ssot GUIDE GEN POST EXPORT TMPL CLIP VOICE THEME MKT start: from ideation/campaign-reviewer-focus (autopilot v7k2).
- 261015 start-vibe v7k2 autopilot start: update-ssot from ideation/campaign-reviewer-focus first; cfg push main.
- 261015 ideation campaign-reviewer-focus ready: always-on principle groups (facts, impressions + constructive honesty, no AI tells, titles/tags), advertiser guide with checklist and a 광고주 요청 origin color, 체험단 글 switch with a top disclosure placeholder, branching template wizard with owner headings, chat voice learning, four channel menus + 내 설정, clip upload text, 9:16 only, Tistory/site/Markdown out.
- 261015 ideation campaign-reviewer-focus start: product philosophy (체험단 automation, fast monetization, 상생), always-on guidelines, guided template wizard, real-speech voice, per-channel outputs and menus (Naver blog, Naver clip, Instagram clip, Instagram post).
- 261013 ideation friendly-model-names paused (open): grade + model-wide neutral alias (가성비 · 소나무), auto-assigned and operator-editable, never reused; operator-confirmed succession moves the alias to a new version and same-grade eligible selections follow with a notice; per-post credits/badges in options, 자세히 under the field. Open: the alias word list.
- 261013 ideation friendly-model-names start: users see grade+number names (무료1, 가성비2) instead of full model names/versions; details only on request.
- 261013 6e5039c8 pushed (T676 + media-release sampling fix): CI ok, Deploy backend ok, Verify media ok (image, release, worker); production runs 6e5039c8.
- 261013 media-release.py resource sampling: an exec failure is judged by the container's state, not Docker's wording (409 attach upgrade and runc procReady forms seen locally); two full gate runs passed in both layouts.
- 261013 T676 done on main: the title opens with its search words and closes with a short implicative phrase.
- 261013 T676 start on main at 6fe0333c: titles 기본 지침 first two sentences.
- 261013 create-task GEN r30: T676 (titles 기본 지침 opens with search words, then an implicative phrase).
- 261013 update-ssot GEN r30 done: GEN-49✎ the title opens with the search words the material supports (area/station, business or product, category or key menu) and closes with a short implicative phrase; GUIDE-41 and TMPL-52 only reference it; no doing task; next create-task GEN.
- 261013 update-ssot GEN start: implicative-only titles drop the post's search terms.
- 261012 76b619df pushed from this tree mid-gate; local gate on it passed (CI list, Compose, 7 images, worker smoke, release in both layouts; usage/store and clip/store load flakes passed on rerun). Remote: CI ok, Deploy backend ok, Verify media Media worker failed at CPU parity like c01b5169.
- 261012 T675 done on main: 최소 태그 수 / Minimum tag count everywhere; the tag help now explains the minimum and the broad-label tail.
- 261012 T675 start on main at efc7f59a: 최대 태그 수 copy becomes 최소 태그 수.
- 261012 T674 done on main: tag_count is a minimum the prompts ask for, tags kept up to 30, broad labels fill the tail, taste inferred from inputs as experience; write/revise no-voice system prompts shrank 148/163 runes.
- 261012 T674 start on main at 8fe42b0e: tag minimum, broad-label tail, inferred taste, prompt rules edited in place.
- 261012 create-task GEN r29 POST r40 TMPL r27 GUIDE r21: T674 (BE prompts, parser, 기본 지침), T675 (FE 최소 태그 수 copy); no dep between them.
