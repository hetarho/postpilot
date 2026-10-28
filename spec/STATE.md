# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en
- push: main

## inbox
- 261028 decided TMPL-73: how does an owner style request meet the 기본 voice? → the request decides wording and outranks the voice, which still decides without one; examples are code-owned chips that fill the field (alt: the request replaces the voice entirely | free text only, no examples)
- 261028 decided TMPL-71: what replaces the top 내 말로 쓰기 button on the headings step? → the frame's ordinary 다음 in the next-action place, the heading fields being the own-words input (alt: keep the 내 말로 쓰기 label in the next slot)
- 261028 decided TMPL-32: how does a writer with no template get one? → `새 템플릿 만들기` in ①'s template picker opens the wizard and returns to the post with it assigned (alt: link out with no return | a hint line under the picker)
- 261028 suggest TMPL-73 CLIP-213: the clip template wizard still offers the direct suggestion button; give it the same style-request sheet if wanted
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
| POST | 42 | 42 | - | 0 |
| VOICE | 17 | 17 | - | 0 |
| GEN | 31 | 31 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 31 | 31 | - | 0 |
| GUIDE | 23 | 23 | - | 0 |
| EXPORT | 12 | 12 | - | 0 |
| LANG | 10 | 10 | - | 0 |
| THEME | 34 | 34 | - | 2 |
| MKT | 11 | 11 | - | 0 |
| VIDEO | 8 | 8 | - | 0 |
| CLIP | 64 | 64 | - | 2 |
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
| T709 | Prove the real clip pipeline within the approved live budget | ARCH CLIP QUOTA | T708 | blocked@261028 |
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |
| T697 | Expose Content-Range on private storage and gate the deploy on it | ARCH INFRA | - | blocked@261016 |

## next
- TMPL r29–r31 tasks T710–T715 are done and pushed; watch the pushed CI/deploy/media runs.
- T708 is deployed and d3d51209 CI/Workers plus actual retained-original playback/preview pass. T709 fresh planning remains blocked by unresolved provider usage; preserve its ledger and keep T603/T604/voice gates closed.
- T697 deployment/CORS/media gates passed at177b377a; deployed browser preview now reads8 real retained R2 originals without a range failure. Full public browser-generation qualification remains gated by T603/T604.

## log
- 261029 T715 done: one content-kind card with horizontal radios named by outcome; its drawings and inputs switch inside it.
- 261029 update-ssot TMPL r31 (owner) + create-task T715: wizard content kinds renamed by outcome; each card holds its own inputs.
- 261029 T714 done: the wizard's 직접 입력 is a question the AI writes the part from; card, preview and copy follow.
- 261029 update-ssot TMPL r30 (owner): 직접 입력 becomes a question whose answer the AI writes the section from, replacing the verbatim answer (resolves the 261015 inbox decision).
- 261029 T713 done: ①'s 템플릿 picker offers 새 템플릿 만들기; the wizard returns to the post (or a new draft) and the editor assigns the new template once.
- 261029 T712 done: 제목 제안 받기 style-request sheet with examples, ordinary 다음 on the headings step, content-kind cards drawing 글 쓸 때/완성된 글; checked in real Chrome at desktop and phone widths.
- 261029 T711 done: compact preview stand-ins shared by TemplatePreview and the new WizardPreview; wizard preview beside every step from sections on (lg sticky right, 미리보기 sheet below lg).
- 261029 T710 done: heading suggestions carry an optional owner style request (proto field 5, ≤200, outranks the 기본 voice); template/cmd/api Go tests and affected FE tests pass.
- 261029 create-task TMPL r29: T710 heading style request (proto/BE/hook), T711 compact preview + wizard side preview, T712 style sheet + 다음 + content-kind cards, T713 picker 새 템플릿 만들기.
- 261028 update-ssot TMPL r29: wizard heading style-request sheet, preview beside every step (lg right / 미리보기 sheet), content-kind cards with 글 쓸 때·완성된 글 panels, compact preview stand-ins, 새 템플릿 만들기 in ①'s picker. Audit: first-use /setup and the voice picker's 새 말투 만들기 exist; the template picker had no creation path.
- 261028 update-ssot TMPL start: template-creation UX friendliness (owner request).
- 261028 fix clip delivery: rerendered the latest completed clip from the identical retained production plan/originals with exactly one intro headline changed; private1080x1920 H264 High/30fps/900-frame/30s output passes production validation, independent full decode and Chrome seek/playback. Owner download and private server copy retained; completed project/result unchanged.
- 261028 start introfix: inspect the latest server video and rerender with the owner-supplied intro business name only.
- 261028 delivery verified atd3d51209: CI40546530178 frontend562/4480 and full backend pass; Workers7ccde8a4/versionf3298eab and live-byte/native-playback/deployed-R2-preview checks pass. Final documents reuse unchanged source verification; private videos and offline review page are retained, with T709 usage/corpus/human-review gates still open.
- 261028 d3d51209 pushed after the authorized playback allowance; Workers7ccde8a4 succeeds with versionf3298eab. Live entry/clip/preview-worker/config bytes match. Actual HEVC originals play and deployed R2 preview completes24 samples in45.034s plus frames0/300/600/899; full CI is still running.
- 261028 T709 blocked:8 real browser observations confirm USD0.010868; the next write request has missing usage, so its USD0.025001 reservation remains unresolved and further paid work stops.24 full-verified analysis artifacts and2 independently decoded/Chrome-played1080x1920/30fps/30s exports of the prior real AI plan are retained privately. No fresh full-pipeline or qualification claim; source credentials removed, human review requested.
- 261028 T709 starts (prv, explore2h): pursue real browser/native copy comparison and repeated metered OpenRouter-to-playable-video proof under the owner cumulative USD0.20 cap. Source checkpoint c4f3254c passes preview4480 FE tests; no live diagnostic call has yet been sent. Preserve real qualification/human-review gates and the existing user result.
- 261028 T708 done atc4f3254c: finite retained-preview reading, source-end rounding and distinct scene preparation/frame phases pass full4480 FE tests and actual R2/Chrome visual checks. Source and sanitized QA are committed locally; final batch push/CI and authorized cumulative USD0.20 real AI/video proof remain.
- 261028 T708 verified locally: full562-file/4480-test FE suite, production type/build and changed-file lint/format pass. Real R2/Chrome preparation reports24 scene checks in42.238s and frames0/300/600/899 display correctly; owned local bytes take3.472s to first frame. Source checkpoint precedes the authorized budgeted AI/video proof; no push yet.
- 261028 T708 starts (prv): actual Chrome decodes the retained HEVC originals but per-packet R2 reads consume the preview deadline during whole-plan contrast sampling. Restore bounded adjacent reads without changing the composition/output contract or qualification gates.
