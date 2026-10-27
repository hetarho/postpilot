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
| prv | ihaelam-ui-MacBookPro | pair | 2026-10-27T15:57Z | 2026-10-27T15:57Z | T709 |

## inbox
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
| yozm-it-product-adoption | open@261019 |

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
| T709 | Prove the real clip pipeline within the approved live budget | ARCH CLIP QUOTA | T708 | doing@261028.prv |
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |
| T697 | Expose Content-Range on private storage and gate the deploy on it | ARCH INFRA | - | blocked@261016 |

## next
- Continue ideation yozm-it-product-adoption: recommended templates for personal experience, judgment and voice; decide reusable author context versus per-post material.
- Run the SSOT-bound live/video proof within the shared USD0.20 cap after T708 preview recovery; keep unmet release gates closed and record actual outcomes before the final push.
- T697 production ranged-GET/CORS deployment gates and media verification succeeded for177b377a; a real production browser clip source-read check remains. Existing qualification blocks are unchanged.

## log
- 261028 T709 starts (prv, explore2h): pursue real browser/native copy comparison and repeated metered OpenRouter-to-playable-video proof under the owner cumulative USD0.20 cap. Source checkpoint c4f3254c passes preview4480 FE tests; no live diagnostic call has yet been sent. Preserve real qualification/human-review gates and the existing user result.
- 261028 T708 done atc4f3254c: finite retained-preview reading, source-end rounding and distinct scene preparation/frame phases pass full4480 FE tests and actual R2/Chrome visual checks. Source and sanitized QA are committed locally; final batch push/CI and authorized cumulative USD0.20 real AI/video proof remain.
- 261028 T708 verified locally: full562-file/4480-test FE suite, production type/build and changed-file lint/format pass. Real R2/Chrome preparation reports24 scene checks in42.238s and frames0/300/600/899 display correctly; owned local bytes take3.472s to first frame. Source checkpoint precedes the authorized budgeted AI/video proof; no push yet.
- 261028 T708 starts (prv): actual Chrome decodes the retained HEVC originals but per-packet R2 reads consume the preview deadline during whole-plan contrast sampling. Restore bounded adjacent reads without changing the composition/output contract or qualification gates.
- 261028 fix opaque GPU video frames: query an explicit RGBA copy-layout size without copying or replacing decoded frames; retain presentation reservations and cleanup. Actual Mac Chrome HEVC Main10 decoding exposed format=null; production preview previously threw allocationSize. Focused21 and affected345-file/2820-test checks plus production type/build pass. A separate remote-read latency issue remains under investigation; no preview completion claim or push yet.
- 261028 owner expands preview recovery to SSOT-compliant browser analysis and end-to-end video evidence, authorizes cumulative OpenRouter spend at most USD0.20 including retries, and requests commit/push only after current playback finishes. Owner subsequently allows push after30 minutes; earliest push2026-10-27T16:33:30Z (01:33:30 KST). Continue local implementation and verification.
- 261028 clip preview recovery start (prv, pair): owner reports original playback and edited composition preview both fail after native analysis recovery; inspect the affected production project and retained source access through PEM, then reproduce the browser playback failure. Existing fix/deploy authorization applies; preserve unrelated ideation changes.
- 261028 T707 done at310959b7: owner selects existing-server recovery; CI40530990767 frontend/backend and Workers fc8d3803 pass. Production assets and explicit native approval are verified. Read-only PEM inspection confirms the owner-approved native job58641d7d completed all8 source analyses and persisted4 storyline paragraphs at15:47:48 UTC; final video rendering was not observed. Browser/voice qualification gates remain closed.
- 261028 T707 deployed at310959b7: Workers fc8d3803 succeeded, live entry/clip/public-config bytes match the tested build, and production-origin Chromium confirms both server-labelled approvals with one native start each and no browser preparation request against an isolated mocked API. Full remote CI remains running; actual affected-original/provider completion needs a fresh owner approval and is not claimed.
- 261028 T707 local verification:560 FE files/4464 tests and affected native app/store/worker/RPC checks pass; final disclosure copy passes its source-picker/page/workspace regressions and lint/build. Built-app Chromium confirms visible server approval and exactly one native start with no analysis preparation ID for storyline/direct actions against a mocked API; actual owner-approved provider completion remains unverified. Unchanged full backend/deploy/generated evidence is reused; frontend-only diff triggers no backend/media deployment.
- 261028 T707 starts at7ffe6fa6 (avf): select native preparation before approval when browser readiness is closed; enforce server-returned qualification before browser activation and retain independent export/quality gates.
- 261028 create-task CLIP r64: T707 restores explicitly approved native preparation and early browser readiness checks; consumes CLIP-220. Production native runtime is ready, while browser copies have no verifier and the quality gate remains closed.
- 261028 update-ssot CLIP start (avf): browser-analysis readiness is disabled while the workspace always starts it; restore an explicit qualified server-preparation approval, preserving browser quality and export gates. Optional route-preference question remains unanswered; proceeding with the recommended existing-server recovery under the authorized fix/deploy scope.
- 261027 analysis verification stall start (avf, pair): owner reports the deployed clip flow remains at browser analysis verification after the source measurement repair; trace the preparation workflow and inspect the PEM production host, preserving unrelated ideation changes.
- 261027 clip production verification:42c57c1b native build b42c8c7f-85d0-47b1-97bf-8110456e852c succeeded (version31a96f6e-b945-4471-bde2-5337af6ff832) without new OAuth or toolchain changes; GitHub CI40504156313 passed for the exact deployed revision. The live screen now loads analysis.worker-CI2qrO_c.js, byte-identical to the verified artifact (SHA256 f566c4cf5ac708ebec90ffa611a3439e813dd94d792b401456d2c92a4ed41689); Chromium measured300 VFR frames and encoded/inspected150 analysis frames on the production origin. Public configuration is unchanged. Evidence: /tmp/postpilot-cf-recovery-261027/production-worker-verification.json.
- 261027 clip delivery retry: owner requires PEM inspection and rejects additional Cloudflare OAuth; no further login attempts. GitHub check-run rerequest returns404 with existing repository credentials. Publish this operational record once through the existing main integration after the matching queue incident resolved; inspect all exact-revision checks and production worker bytes before claiming deployment.
- 261027 build diagnosis: owner-provided b586b831 log stops at Installing nodejs24.18.0 / error occurred while installing tools or dependencies, before project build/deploy. Its00:34-00:53 KST initialization overlaps the resolved00:12-01:03 Cloudflare queue incident https://www.cloudflarestatus.com/incidents/dv2vbq9kbgxn; correlation does not establish the Node installer root cause. Node24.18.0 official Linux archives exist and the same version passed local/remote CI; preserve the toolchain.
- 261027 PEM recovery evidence (cfd, pair): api.postpilot.haeram.me health and the CPU worker are healthy; frontend remains on Cloudflare with old analysis.worker-6tVvb6fF.js. Production-config-matched build, compiled VFR measurement/encoding and Wrangler dry-run pass for analysis.worker-CI2qrO_c.js (artifacts: /tmp/postpilot-cf-recovery-261027). Documentation-only retry reuses unchanged3432e285 full-CI evidence (4459 FE tests/89 BE packages/68 deploy tests); no backend/media deployment paths changed.
- 261019 ideation correction: inspect TMPL r28, VOICE r17 and wizard/answer code; voice learning already exists, while composition recommendation and experience-field defaults remain proposals.
- 261019 ideation yozm-it-product-adoption: owner favors automatic composition recommendations and distinctly personal output; record goals and unresolved collection boundaries without changing product policy.
- 261019 ideation yozm-it-product-adoption start (i9t2, pair); identify the source article before exploring product applications.
