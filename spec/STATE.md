# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en
- push: main

## inbox
- 261109 decided review/perf-quality-si4-261109 T742: canonical FFmpeg TLS blocks the cold build after T741 recovers dependencies → use measured identical-source/original-signature/official-key fallbacks (alt: change source/version | weaken TLS or GPG | count old images as passed) — same archive SHA and full signer verified inside pinned base; only download availability changes.
- 261109 decided review/perf-quality-si4-261109 T741: old zlib pin disappeared from the current index → retain the generic install and recover identical signed official APKs with fixed checksums (alt: upgrade runtime | pretend old image tags pass | weaken media gates) — actual same-version install proven; only two packaging paths, original architecture capability/trust/contracts preserved.
- 261109 decided review/perf-quality-si4-261109 T740: observed VM clock rollback rejects the active attempt checkpoint → repair SQL ownership using the existing active-job index and atomic checkpoint replacement trigger (alt: retry tests | change timestamps/clocks | weaken the gate) — deterministic and real-run proof, no policy/schema/provider/credit change; this repair precedes the pending source publication.
- 261109 decided review/perf-quality-si4-261109 T737 T738 T739: four-hour local improvement delegation → approve measured template/guideline focus recovery and an exact 20-file Node test project (alt: broad DOM/test refactors | dependency/policy changes) — five real-route BODY defects and 31.22→2.73 s paired 83-case subset; preserve existing contracts, independently review every unit, final full CI before cfg main push.
- 261109 decided review/perf-quality-261109 T736: third round → adopt F11 /memories keyboard focus for /memories only, reject F4 after re-measuring (2.5 s with T733) (alt: also templates, clip templates and guidelines | stop the run) — measured defect, one page, same rule as T735; the other directories' shape is inferred and their extension stays a scope choice.
- 261109 decided review/perf-quality-261109 T735: second scout round (interaction UI/UX) → adopt F10 Editable keyboard focus, defer F11 /memories focus, reject F6 useJob error polling (alt: F11 too | fix F6) — F10 is measured on the core ② 다듬기 flow and invisible to pointer users; F11 has smaller reach and an open scope choice; F6 has no user-facing trigger.
- 261109 decided review/perf-quality-261109 T733 T734: which scout proposals does this local improvement run (owner: until 08:00, focus performance, code quality and UI/UX) implement? → F1 per-binary template DB for Go tests and F2 lazy post editor routes, both behavior-preserving with measured evidence (alt: add F3 lazy motion | add F4 per-iteration queue stop) — F1 is ≈60% of CI's backend job and F2 a one-file cut of every first load; F3 changes the first-moment segmented animation, F4 should be re-measured after F1, and both would push the review, pre-push and media gates past 08:00.
- 261109 decided review/perf-quality-261109: F5 i18n split, F7 CI entry-size budget, F8 sqlc image pin and F9 test-setup barrel imports → kept [?] for the owner or a later run (alt: implement now) — F5/F7 change an ARCH rule or CI gate, F8 is an ARCH-33 dependency change, F9 showed no wall gain in a 57-file A/B.
- 261109 decided T734: preload the editor chunk at idle after the signed-in shell mounts? → no; the first editor open shows the existing RoutePending like every other lazy page (alt: idle preload from AuthenticatedLayout) — smallest change, no new side effect.
- 261103 decided CLIP-224: a model-written caption refused at render start (prod narration-2, repeated) → free it to automatic placement, then omit it with a notice, save the next revision and render (alt: keep refusing and ask the owner to fix it | repair only at generation, leaving saved plans broken) — the owner asked that captions never block a render and that hallucinated output be absorbed; owner-edited content still refuses.
- 261102 decided ARCH-68 CLIP-155: browser final rendering is closed by its readiness record and the API flag, so ② refuses it as 준비 중 — open it? → open it now; the unmet performance, Windows integrated-GPU, lower-resource, human-review and WASM license-graph gates become follow-up evidence, and analysis preparation stays closed (alt: keep it closed until T604 qualifies it | open it behind an operator env) — the owner asked for it to work, the editor preview already ships the same components and WASM, and code-owned activation keeps one switch on both sides.
- 261102 decided POST-121 CLIP-223: where does the nudge to make a missing voice or template appear? → when a new post or clip opens on an account missing it, in a sheet, after the directory reads succeed and before anything is typed (alt: on every creation button, which misses home, setup and links | a note under the picker, which the owner rejected for simple controls)
- 261102 decided POST-121 CLIP-223: how often does it come back? → on every later new post/clip while the kind is still missing, with `이대로 쓰기` for this creation and `다시 묻지 않기` per account on this browser; setup's finish and a return from the offer's own wizard do not ask again (alt: once per account like first-use setup, which never asks again after the last voice is deleted | every time with no opt-out)
- 261102 decided POST-121: what does a voice still being made count as? → missing, but the action becomes `말투 이어서 만들기` and opens that voice's learning (alt: present, so no offer | 새 말투 만들기, leaving the first one half made)
- 261102 decided CLIP-223 CLIP-146: what does a new clip offer? → only a video template, whose wizard returns to the new clip with it selected; never 말투, because a clip takes no writing voice (alt: offer 말투 too | link to the wizard with no return)
- 261030 decided CDS-70 CLIP-111: remove the A/B starting intro/outro designs → drop the candidate letters from the names (심플 제목 · 위아래 가로선 · 가로선 구분 · 점수 강조), keeping the presets and their ids a/b/e (alt: delete presets a/b/e and migrate stored selections | keep the lettered names)
- 261030 decided CLIP-222: captions, intro and outro unseen in the template preview → frame first, an 인트로 · 자막 · 아웃트로 switch, and design choices move the scrubber to their part (alt: three fixed frames side by side | autoplay the scrubber)
- 261030 decided THEME-38 VOICE-52: where does 말투 live once it is a channel section? → one /voices section shared by all three channels, the URL carrying a clip channel (none ⇒ 네이버 블로그), and 내 설정 no longer lists it (alt: keep it in 내 설정 too | a per-channel copy) — one home keeps one parent and one back link.
- 261030 decided THEME-38: which pages show the section tab row? → the four section list pages only; editor, clip workspace, wizards and a voice's own pages show the main row only (alt: every channel page) — no extra chrome while writing or editing, matching the creation menu's pages.
- 261030 decided THEME-67: what does 새 말투 / 새 지침 open? → the section page with its creation sheet open (`?new=1`); other creation choices (AI 말투, 기본 지침, 직접 작성) sit in the page above the list (alt: keep them in a second dock, which THEME-24 forbids beside the menu)
- 261030 decided THEME-58: with the breadcrumb removed at every width, how does a child route show its way back? → one back link at the top of its content naming its actual destination; editor, clip workspace and writing tests keep their own top-row link instead (alt: browser back only | keep the desktop breadcrumb row) — THEME-58's deterministic return survives without a second navigation row.
- 261030 decided POST-43 CLIP-41: what happens to /posts and /clips?channel= once the channel itself is the history? → they replace themselves with /naver-blog, /naver-clip or /insta-clip keeping their filters (alt: both addresses render the same list | drop them) — bookmarks and old return links keep working with one canonical address.
- 261030 decided THEME-67: does desktop get the same floating action menu? → yes, at every width in THEME-24's add-action dock place (alt: inline buttons above the list on desktop) — THEME-24 already keeps a list's add action docked at every width.
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
| ARCH | 21 | 21 | - | 0 |
| AUTH | 15 | 15 | - | 0 |
| QUOTA | 38 | 38 | - | 0 |
| POST | 45 | 45 | - | 0 |
| VOICE | 19 | 19 | - | 0 |
| GEN | 31 | 31 | - | 0 |
| MODEL | 36 | 36 | - | 0 |
| TMPL | 31 | 31 | - | 0 |
| GUIDE | 25 | 25 | - | 0 |
| EXPORT | 12 | 12 | - | 0 |
| LANG | 10 | 10 | - | 0 |
| THEME | 38 | 38 | - | 2 |
| MKT | 11 | 11 | - | 0 |
| VIDEO | 8 | 8 | - | 0 |
| CLIP | 72 | 72 | - | 2 |
| CDS | 38 | 38 | - | 1 |
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
| perf-quality-261109 | converted@261109 |
| perf-quality-si4-261109 | converted@261109 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|
| T709 | Prove the real clip pipeline within the approved live budget | ARCH CLIP QUOTA | T708 | blocked@261028 |
| T539 | Qualify the complete Korean voice-creation milestone | ARCH DUB MODEL CDS THEME | T538 | blocked@261005 |
| T550 | Qualify narrated editing, delivery and private asset lifecycle | ARCH DUB CLIP MODEL QUOTA CDS THEME | T542 T543 T549 | blocked@261005 |
| T603 | Qualify browser analysis information preservation | ARCH CLIP CDS QUOTA | T588 T601 T602 | blocked@261007 |
| T604 | Qualify browser rendering and static deployment | ARCH CLIP CDS INFRA | T588 T590 T599 T600 T601 T602 | blocked@261007 |
| T697 | Expose Content-Range on private storage and gate the deploy on it | ARCH INFRA | - | blocked@261016 |
| T731 | Make the source cadence measurement deterministic | CLIP ARCH | - | todo |
| T739 | Avoid browser setup for audited logic tests | ARCH | - | blocked@261109 |
| T740 | Keep attempt checkpoint ownership independent of wall-clock order | ARCH CLIP | - | todo |
| T741 | Recover pinned zlib build artifacts after index rotation | ARCH | - | todo |
| T742 | Keep pinned FFmpeg downloads available during an origin outage | ARCH | - | todo |

## next
- si4 local main integrates reviewedT737/T738/T739; final actual-main full FE passes580files/4638tests plus lint/format/FSD/type/build. T737/T738 are locally done; T739 is blocked on its explicit full-CI completion acceptance, with config already implemented. Do not reimplement it.
- Every source push and whole-group completion remain blocked: fresh full CI378fe54a fails unchanged clip/store continuation test with CLIP_PROCESSING_FAILED. Preserve the failed native receipt and original full-CI group gate; do not substitute an older Go pass. Report reports/261109-si4.md.
- T740/T741/T742 are deferred after real92daffbe release failure; reviewed source/evidence remain on archive/si4-backend-candidate-261109. Rollback9fa63516 restores the backend exactly toorigin0cd149ca. They are not completed product changes.
- Resume backend diagnosis only with a new bounded scope/time reservation. Existing T731 and blocked qualification/planning work remain outside this run; no automatic continuation or standing schedule is enabled.

## log
- 261109 si4 local closure: reviewedT739 merge72ab/T738 mergef127; actualmain580files/4638tests andallFEchecks pass with stabledotenv. ArchiveT738 underARCH24; blockT739/wholegroup/sourcepush on observedGo failure, preservebackendrollback/archive and olddirtyspaces; endbefore21:00KST.
- 261109 si4 defersT740/T741/T742: actual release failed after13 passed image/worker stages; observed10.223s Docker clock rollback and latest-job query reproduction, final render cause unresolved. Preserve92daffbe/archive branch and restore backend exactly toorigin0cd at9fa63516; finish FE scope before21:00KST.
- 261109 create-task review/perf-quality-si4-261109 F7 (si4) → T742 same-byte authenticated FFmpeg input fallback; source/signature/key identity proven in pinned base, no gate or runtime change.
- 261109 si4 deadline clarified by owner: complete before21:00KST; T741 reviewed source integrated, final media gates then staged T739/T738 publication remain.
- 261109 create-task review/perf-quality-si4-261109 F6 (si4) → T741 same-version signed/checksummed APK fallback; no library upgrade or gate relaxation. Both original media build failures preserved.
- 261109 T740 done (si4): reviewed c3423b1b, source a89b15b7/submission d796f89b merged6a7bd326; three affected packages and17 regression subcases pass; full CI/media gate before pending T737/source publication.
- 261109 T740 replan (si4): first gate exposed stale trigger assumption; release340e69d1 before clarifying current0162 (upload-text retains, sampling clears), preserve source fdcc92b7; SQL ownership fix and schema unchanged.
- 261109 create-task review/perf-quality-si4-261109 F5 (si4) → T740 checkpoint ownership independent of wall clock; actual 10-second rollback and deterministic SQL probe reproduce Go failures; publication held for repair.
- 261109 T737 done (si4): approved review 76ed0387; source integrated 7e1f30c5; unit evidence reuses identical verified source 99399408 (34 affected tests plus 54 consumer cases, six base failures); per-unit full CI/push next.
- 261109 create-task review/perf-quality-si4-261109 (si4) → T737 template-directory focus helper/consumers, T738 guideline-directory focus, T739 exact 20-file Node project; preserve existing contracts.
- 261109 self-improve-local starts (si4): user delegates a roughly four-hour run on main at 97e07304, prioritizing performance, test cost, code quality and keyboard usability; proposals require orchestrator approval and independent code review.
- 261109 pushed c96bc898 (llm sentinel fix + report, sil): backend gate at 534ad96e passes (90 packages, 119 s); CI 42522650704, Deploy backend 42522650731, Verify media 42523039487 success.
- 261109 fix llm tests: leak checks searched inspection JSON for short numeric sentinels ("999", "90000", "0.7") that a live RFC3339Nano IssuedAt can contain — CI 42520643603 failed on spec-only 1115efa7; sentinels now cannot match a timestamp (15/3000 and 2357/250000 failures → 0); reviewed approved (sil).
- 261109 pushed fb01e678 (T736 + report, sil): FE gate at df31babb (577 files/4,592 tests, all FE checks) passes; CI 42519697954 success; no backend path, so no deploy.
- 261109 pushed 5c2a8c03 (T733 T734 T735, sil): full CI gate at 5c2a8c03 and media/deploy gates at ee9ef3fa pass; CI 42516751893, Deploy backend 42516751884, Verify media 42517082146 success; CI backend job 1154→385 s.
- 261109 T736 integrated (sil): /memories keeps keyboard focus after an edit closes or a delete (only while focus is lost, once per removal); 3 cases fail at base; reviewed approved (review 4279acaf, P3 only).
- 261109 create-task review/perf-quality-261109 F11 (sil) → T736 /memories keyboard focus; F4 rejected after re-measurement.
- 261109 T735 integrated (sil): Editable keeps keyboard focus on entry/exit (keyboard-only entry move, exit only while focus is lost); 3 cases fail at base; reviewed approved (review ff709665, P3 only).
- 261109 create-task review/perf-quality-261109 F10 (sil) → T735 Editable keyboard focus; F6 rejected, F11 kept [?].
- 261109 T733 integrated (sil): DB tests copy a once-migrated template (74 of 86 sites); post/store 117.6→3.5 s locally; reviewed approved (review 4d401114, P3 only).
