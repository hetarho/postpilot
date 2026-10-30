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
| dgc | DESKTOP-VKPLS8P | autopilot | 2026-10-30T02:11Z | 2026-10-30T02:11Z | - |
| chl | DESKTOP-VKPLS8P | autopilot | 2026-10-30T02:42Z | 2026-10-30T02:54Z | T722 |

## inbox
- 261030 decided THEME-58: with the breadcrumb removed at every width, how does a child route show its way back? → one back link at the top of its content naming its actual destination; editor, clip workspace and writing tests keep their own top-row link instead (alt: browser back only | keep the desktop breadcrumb row) — THEME-58's deterministic return survives without a second navigation row.
- 261030 decided POST-43 CLIP-41: what happens to /posts and /clips?channel= once the channel itself is the history? → they replace themselves with /naver-blog, /naver-clip or /insta-clip keeping their filters (alt: both addresses render the same list | drop them) — bookmarks and old return links keep working with one canonical address.
- 261030 decided THEME-67: does desktop get the same floating action menu? → yes, at every width in THEME-24's add-action dock place (alt: inline buttons above the list on desktop) — THEME-24 already keeps a list's add action docked at every width.
- 261030 decided THEME-38: where does the menu's 말투 item lead? → the existing 내 설정 말투 page with the channel as contextual return (alt: a per-channel copy of 말투) — 말투 is shared by every channel.
- 261030 decided THEME-38: visible channel labels and placement? → 블로그 · 클립 · 인스타 beside the brand on desktop, in one persistent row on phones; settings stays in the account menu (alt: hamburger-only | four primary destinations) — immediate channel discovery with room for the current location.
- 261030 decided THEME-38: shared video settings context? → channel in links and child links (alt: only remembered browser context) — new tabs, copied links and reload retain Instagram.
- 261029 decided CLIP-221: how does the video-template wizard's live preview show a stage name or order the clip frame never draws? → an ordered stage list above the frame marks the stage whose step is on screen, as the post wizard marks its section (alt: the frame alone, so renaming or reordering stages changes nothing visible | draw stage names into the sample clip as captions)
- 261029 decided CLIP-221: does the live preview offer the design selection on every step? → before the design step it draws the draft's presets and caption styles without offering them, and the design step's content is the same preview with them offered (alt: offer presets and styles on every step, breaking one question per screen | hide the design until the design step, so the preview draws defaults the owner never chose)
- 261029 decided CDS-35 T717: what does a browser render do when one gain cannot reach −16 LUFS under the −1.5 dBTP ceiling? → apply dynamic peak limiting to the same measured target, as the server loudnorm pass already does by falling back to dynamic mode (alt: keep refusing, leaving source-sound browser renders unusable on real phone audio | lower the loudness target for browser renders)
- 261029 decided CLIP-204: how do browser analysis copies keep audio on Windows, whose Chrome/Edge AAC encoder takes only 96/128/160/192 kbps? → browser copies use AAC mono 96 kbps on every platform, above the server copy's 64 kbps, under a new profile identity clip-browser-analysis-v2 (alt: bundle the @mediabunny/aac-encoder libavcodec WASM to keep 64 kbps, adding ~1 MB and an LGPL component | let Windows fall back to the server route)
- 261029 decided CLIP-219: how does a server render of HLG footage stay SDR? → relabel the delivered encode BT.709 primaries/transfer and keep the coded matrix, pixels unchanged, matching the browser preview's reading (alt: convert the matrix to BT.709 in the graph, changing pixels | re-add the r62 HDR→SDR conversion the owner cut)
- 261029 decided ARCH-68 CLIP-155: what does ② offer while browser final rendering is unqualified? → the browser kind stays listed, refused by name, and the server leads (alt: hide the browser kind | keep offering it and let the server refuse)
- 261028 decided TMPL-73: how does an owner style request meet the 기본 voice? → the request decides wording and outranks the voice, which still decides without one; examples are code-owned chips that fill the field (alt: the request replaces the voice entirely | free text only, no examples)
- 261028 decided TMPL-71: what replaces the top 내 말로 쓰기 button on the headings step? → the frame's ordinary 다음 in the next-action place, the heading fields being the own-words input (alt: keep the 내 말로 쓰기 label in the next slot)
- 261028 decided TMPL-32: how does a writer with no template get one? → `새 템플릿 만들기` in ①'s template picker opens the wizard and returns to the post with it assigned (alt: link out with no return | a hint line under the picker)
- 261016 suggest T694 T695 T698 T699 T700: review-code over channel navigation, campaign positioning, owned post links, frozen writing material and original capture metadata.
- 261016 decided T700: how is EXIF without a UTC offset normalized? → omit the unknown instant and retain filename-based order as fallback (alt: invent the browser’s current time zone | label the local wall clock as UTC); use bounded mvhd parsing because the installed Mediabunny skips that creation field.
- 261016 decided T699: how does a clip-store transaction fence external post text? → freeze a same-read creation identity and carry its text digest in pricing beside GuidelinesDigest, with the raw link identity in the project digest (alt: read post tables from the clip context | persist live post text on every project edit).
- 261016 decided T698: how is T692’s existing creation seed linked atomically? → optional linked_post_slug on CreateClipProject as well as Update; the owned canonical post is validated before insertion (alt: create, then a second seed/link update that can leave a partial project). Picker actions save only the link through the write lane.
- 261019 parked T697: implementation is on main;177b377a CI, backend rollout (including ranged-GET/CORS gates) and all3 media jobs succeeded. Real production browser clip source-read qualification remains unverified; the earlier interrupted local Go sweep is not a passing gate.
- 261016 suggest T690 T691 T705 T692 T706: review-code over campaign guidance, upload text, partial imports, post-to-clip creation and fixture synchronization
- 261016 decided T705 T692: how can a failed video transfer keep the clips already uploaded? → an owned initial-import completion primitive moves verified leases into a ready batch and cleans only pending uploads under existing retention rules (alt: discard every arrived video | re-upload successful files repeatedly | leave a non-ready batch)
- 261016 decided T691: where does a source-free upload-text approval live? → its own project-owned quote table, leaving source-bound generation quotes intact (alt: nullable batch identity in the existing table | require retained originals)
- 261016 decided T691: what if required advertiser hashtags exceed the platform limit? → refuse before quote/provider/credit work and name the limit, preserving every required tag (alt: silently discard required tags | produce text the platform refuses)
- 261016 suggest MODEL-9 MODEL-19 GUIDE-41: doc-review MODEL/TMPL/GUIDE over the existing quality candidates reported by spec lint; keep policy changes in update-ssot
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
| POST | 43 | 43 | - | 0 |
| VOICE | 17 | 17 | - | 0 |
| GEN | 31 | 31 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 31 | 31 | - | 0 |
| GUIDE | 23 | 23 | - | 0 |
| EXPORT | 12 | 12 | - | 0 |
| LANG | 10 | 10 | - | 0 |
| THEME | 36 | 36 | - | 2 |
| MKT | 11 | 11 | - | 0 |
| VIDEO | 8 | 8 | - | 0 |
| CLIP | 67 | 67 | - | 2 |
| CDS | 37 | 37 | - | 1 |
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
| T709 | Prove the real clip pipeline within the approved live budget | ARCH CLIP QUOTA | T708 | blocked@261028 |
| T721 | Shared floating action menu | THEME | - | doing@261030.chl |
| T722 | Channels open on their history with the floating action menu | THEME POST CLIP | T721 | doing@261030.chl |
| T723 | Breadcrumb-free shell with header channel links | THEME | T722 | todo |
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |
| T697 | Expose Content-Range on private storage and gate the deploy on it | ARCH INFRA | - | blocked@261016 |

## next
- implement-task T721 (chl), then T722 and T723: floating action menu, channels open on their history, breadcrumb-free shell.
- T720 and the compact technical-action fix complete: three persistent channel links, account settings and URL-carried video context. Local final gate: 568 files/4509 tests plus all CI-equivalent FE checks pass; report reports/261030-nux.md.
- T718 (CLIP r66) pushed: the video-template wizard shows the live template preview beside every step from stages on, stages listed with the on-screen one marked, design offered only on the design step. CI run 40836194942 confirmed success; look at it at lg and on a phone; report reports/261029-vtp1.md.
- Windows Chrome browser media fixes shipped at a4fe7473 (CI, Deploy backend and Verify media success; live preview/video workers carry profile v2): T716 analysis AAC 96 kbps, opaque D3D11 frame rendering, T717 loudness limiter. Owner: listen to the limited-audio render; browser analysis/final-render readiness, T603 corpus/semantic review, T604 hardware/human review and the T709 provider reservation stay open.

## log
- 261030 create-task THEME r36 POST r43 CLIP r67 → T721 floating action menu, T722 channel history landing (+/posts, /clips redirects), T723 breadcrumb-free shell with header channel links.
- 261030 update-ssot THEME r36 POST r43 CLIP r67 (chl): THEME-38✎ THEME-58✎ THEME-67+ POST-43✎ CLIP-3✎ CLIP-41✎ channels open on their history, floating action menu (+말투), no breadcrumb, phone links in the header row, underline current mark.
- 261030 update-ssot THEME start (chl, autopilot): channels open on their history with a floating action menu, no breadcrumb, phone channel links in the header row.
- 261030 fix dev stack startup: the Go compile cache is a named volume shared by backend and media-worker, and Dockerfile.dev copies only the health client, so a source edit no longer recreates both containers into a cold compile (API 35 s → 2.8 s, worker 21 s → 1.6–2.0 s across recreated containers; image manifest unchanged by a source edit); deploy/media_dev_test.py 5 tests pass.
- 261030 nux final gate at b6b894c1: 568 files/4509 tests and all FE CI-equivalent checks pass; unchanged backend/generation/deploy evidence reused from bb21f128.
- 261030 fix editor inspection: compact secondary technical action on new/saved posts; 3 affected test files/18 tests, file lint/format, type/build and 8 browser compositions pass.
- 261030 T720 done: persistent Blog/Clip/Instagram navigation, account settings entry and shared video URLs; affected tests, lint/format/type/build and 60 browser combinations plus draft/caret continuity pass.
- 261030 T720 starts (nux) at a15e7bb0; persistent channel navigation and portable shared-video context.
- 261030 create-task THEME r35 → T720: persistent channels and URL-carried shared video context; preserve creation, settings and return contracts.
- 261030 update-ssot THEME start: persistent three-channel navigation and channel-context usability (nux, autopilot).
- 261030 T719 setup regression correction: the shared helper now uses Next; all 23 setup tests and file ESLint/Prettier pass. The full pre-push suite is rerun on the amended candidate.
- 261030 T719 pre-push: full FE suite found 2 setup tests still using the old continue label (4504 passed). Update the shared setup-test helper to Next and verify the setup consumer before the final gate.
- 261030 T719 done: shared heading style-request sheet and choices for post/video; ordinary Next on video headings. Node 24.18.0: 28 files/298 tests, typecheck, lint/format/FSD/style/managed-skill checks pass. Full frontend pre-push gate follows on the committed candidate; unchanged backend/generation/deploy evidence is reused from bb21f128.
- 261030 T719 starts (vhn1) at 0f5995bf; the planning push passed the full pinned-Node frontend gate and reused identical backend/generation/deploy inputs from bb21f128.
- 261030 create-task CLIP-213 TMPL-71/73 → T719: use one heading-suggestion sheet for post and video wizards; accepted the matching inbox suggestion. T718 is already archived done; removed its stale doing row.
- 261029 pushed f8e4ad6e (CLIP r66, T718, report vtp1): pre-push FE gate on e5111a90 passed (568/4506, build, lints); CI run 40836194942 jobs completed with no failed step, conclusion unreadable over public HTML (gh not logged in); Deploy backend/Verify media not triggered.
- 261029 T718 done: ClipWizardPreview beside the video wizard's steps (lg aside, phone WizardPreviewDock moved to shared/ui); CompositionPreview draws handed-in presets read-only without a change handler; related vitest 323 files/2662 tests, lint, lint:fsd, tsc -b pass.
- 261029 T718 starts (vtp1)
- 261029 create-task CLIP r66 → T718: WizardPreviewDock moves to shared UI, ClipWizardPreview (stage list + CompositionPreview, design read-only before the design step), wizard layout mirrors TMPL-75.
- 261029 update-ssot CLIP r66: CLIP-213✎ CLIP-221+ the video-template wizard shows the live template preview beside every step from stages on, as the post wizard does (TMPL-75); before it showed only at the design step.
