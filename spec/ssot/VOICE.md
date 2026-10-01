# VOICE voices
> r6 | An account owns zero or more mutually isolated 말투 (voices), each a fingerprint of its owner's surface habits made only from 학습 글 the owner adds — posts they wrote by hand and answers to the product's prompts — counted by the product, described briefly by one explicit analysis call, read back in plain words, checked against the owner's own answer, and projected into prompts without fallback (invariant I4).

## decisions
- VOICE-1 [o] an account owns zero or more voices (`voices`), the user-facing noun 말투 — several when the owner writes in more than one mood, a calm one and a cheerful one; each voice owns its 학습 글, its current analysis, at most one previous analysis and its 검증 results, all keyed by `(user_id, voice_id)`; no shared account-level analysis, inheritance, copying or fallback: a voice with no 학습 글 holds nothing even when a sibling is made ← a merged analysis can never be separated again
- VOICE-2 [o] active display names are unique within the account (`voices_active_name`) and a tombstone's name reserves nothing; at most one active, made voice is the account's 기본 (partial unique index `voices_one_default`), and an account may have none
- VOICE-3 [o] every store query and RPC is scoped by the authenticated user and by one voice, named explicitly (`voice_id` on analysis, 학습 글, 검증 and comparison requests) or derived from an owned aggregate (a post); no request carries a user id; a foreign voice id is indistinguishable from an unknown one (`NotFound`); a same-account id from another voice is refused — never interchangeable material
- VOICE-4 [o] no voice is created automatically: `adduser`, sign-up and reads never create a voice or an analysis, and a new account starts with none ← a voice holds only prose its owner chose to give it
- VOICE-5 [o] a voice belongs to no 분야 and no template: it is the owner's own voice on a post of any 분야 and any template, and neither a template's text nor a guideline enters its analysis or changes it; no revision path writes voice state ← a fingerprint is surface habit, which holds across topics and forms, while the template decides the post's form (→TMPL-1)
- VOICE-6 [o] a voice's analysis is the fingerprint (→VOICE-24) of the 학습 글 it read with one example sentence per item; nothing else is voice state — no rule list, no override, and no history beyond the one previous analysis (→VOICE-30)
- VOICE-8 [o] a 학습 글 is private to its owner: its full text — and an answered photo prompt's photo — opens for the owner on the voice's 학습 글 tab (→VOICE-64) and reaches no other account, voice or log
- VOICE-9 [o] ListVoices returns the account's voices including tombstones — active first, the 기본 first among them, then by name and id — each carrying whether it is made and, until it is, its readiness (→VOICE-32); the order is part of the contract and the frontend re-applies it after a cache patch; GetVoiceProfile returns the voice summary with its current analysis
- VOICE-10 [o] CreateVoice trims the name, requires 1–`VoiceNameMaxChars` (50) Unicode scalar values (`InvalidArgument` otherwise), refuses an active-name collision (`AlreadyExists`) and creates a Korean voice that is not yet made (`만드는 중`, no analysis); it takes no language, description or 분야, calls no model, and there is no voice-count limit
- VOICE-12 [o] RenameVoice changes the display name of an active or deleted owned voice with the same validation, rewriting no post row and no frozen snapshot; SetDefaultVoice makes one active, made, owned voice the 기본 — atomically clearing the previous one — or clears the 기본 so the account has none, and answers with the whole directory ← the previous 기본 changed too
- VOICE-13 [o] DeleteVoice is a soft delete (`deleted_at`):
  - it refuses a voice that still has a queued or running analysis or 검증 (`FailedPrecondition`)
  - deleting the 기본 or the last voice leaves the account with none
  - no model experiment blocks it: a comparison keeps the projection it froze (→MODEL-31)
  - posts, 학습 글, analyses and 검증 results stay
  - the voice disappears from selectable options at once
  - deleting a tombstone is a no-op
  - there is no hard-delete path ← a tombstone keeps post history renderable without cascading rows away
- VOICE-14 [o] RestoreVoice clears `deleted_at` without enqueueing anything, fails `AlreadyExists` while an active voice holds the same name (rename the tombstone first), and never changes the default
- VOICE-15 [o] a deleted voice stays addressable for display — its analysis, 학습 글 and 검증 results readable — but every change (adding or deleting a 학습 글, 말투 만들기, 다시 분석, 이전 분석으로 되돌리기, 검증, a 말투 반영 비교 start) is refused `FailedPrecondition` before any enqueue or provider call
- VOICE-16 [o] directory reads, lifecycle mutations, gathering (pasting, answering, the readiness meter), page load, polling, copy, export, time and boot make no provider call and enqueue no work (I5); the only provider work a voice starts is 말투 만들기, 다시 분석 and 검증, each an explicit press (말투 반영 비교 →MODEL-67)
- VOICE-20 [o] a pasted 학습 글 is trimmed and must contain at least `SampleMinChars` (200) Unicode characters (the rejection names the measured count); an empty label falls back to the first `LabelFallbackChars` (20) characters; pasting needs no model and enqueues nothing
- VOICE-21 [o] adding or deleting a 학습 글 enqueues nothing:
  - before the voice is made it moves the readiness meter (→VOICE-32)
  - once it is made the analysis stays as it is and the voice shows `새 학습 글 N편 · 다시 분석` after an addition and `학습 글이 바뀌었어요 · 다시 분석` after a deletion
  - a deleted 학습 글's sentences leave the excerpts and the example sentences at once ← the owner took that prose back
- VOICE-22 [o] an analysis reads one snapshot of the voice's 학습 글 and publishes what it read even when a 학습 글 changed meanwhile, which then shows as VOICE-21's notice; nothing repeats the call without the owner's press ← every call is the owner's press and its credits
- VOICE-23 [o] 말투 만들기 (the first analysis, offered at 100%) and 다시 분석 (once made, while the 학습 글 again reach 100%) are one durable `analyze_voice` job on the account's active analyze-stage selection (`analyze 0/1 → 1/1`, the model recorded): the product counts the fingerprint over the snapshot, then one call writes the AI part from the same 학습 글 with labelled separators; non-prose lines stay out of both (→VOICE-61)
- VOICE-24 [o] the fingerprint is eight items the product counts and a short part the AI writes:
  | item | counted |
  |---|---|
  | ① endings | the 다 · 해요 · 습니다 · 기타 mix and the frequent sentence-final strings (`~더라구요`) |
  | ② sentence-final marks | the share of sentences ending in `!` `?` `~` `…`, and repeats such as `!!` |
  | ③ emoji and ㅎㅎ·ㅋㅋ·ㅠㅠ | how often each appears per 100 sentences |
  | ④ shape | average sentence length in characters, sentences per paragraph, whether every sentence takes its own line |
  | ⑤ opening and closing | the recurring first and last lines (`안녕하세요!`) |
  | ⑥ adverbs and phrases | the frequent adverbs and short phrases (진짜 · 완전 · 근데) |
  | ⑦ first person | 저 · 제가 · 나 · 우리 · none |
  | ⑧ headings and lists | how headings and lists are written (emoji, question form, numbering) |
  - the AI writes only what cannot be counted: an overall impression in one or two sentences, when each verbal tic appears, and signature phrases with the topic words (보리밥, 양꼬치) filtered out
  - the handler fails a result missing any AI field
- VOICE-25 [o] a successful analysis becomes the voice's current analysis and the one it replaced becomes the previous (→VOICE-30); the first one makes the voice made, so it can be picked for a post (→POST-23)
- VOICE-26 [o] an analysis is one immutable snapshot: each counted item's value, the AI part, one example sentence per item from the 학습 글 it read (the product picks it for a counted item, the AI cites it for its own), the 학습 글 count and the time; an item the 학습 글 cannot show (e.g. no heading anywhere) is `unknown` and reads 알 수 없음, never 0
- VOICE-27 [o] the product computes every counted item itself and never asks the model to estimate one; the call names every AI field it expects and attaches the embedded schema `internal/voice/schemas/voice_analysis.schema.json` when the resolved model declares `structured_output`, falling back to the prompt alone otherwise
- VOICE-30 [o] after a 다시 분석 the voice offers one `이전 분석으로 되돌리기`, which makes the previous analysis current again and discards the one it replaced; there is no redo and no older history, and a deleted voice offers none ← it exists for a re-analysis the owner dislikes
- VOICE-31 [o] `GetVoiceProfile.active_job_id` exposes the voice's queued or running analysis or 검증 so clients resume polling after navigation or reload
- VOICE-32 [o] until the voice is made, a readiness meter counts its 학습 글 by the fingerprint's own segmentation and shows `말투 학습에 필요한 정보 N% 확보`, then at 100% `이제 말투를 만들 수 있어요` with 말투 만들기:
  - 100% needs `VOICE_READY_SENTENCES` (60) sentences and at least one opening, one description and one closing
  - a pasted post counts as all three parts; an answer counts as its prompt's part (→VOICE-60)
  - N is the sentence share, held below 100 while a part is missing, and the meter names the missing part
  - a voice not yet made cannot be picked for a post or made 기본 ← the analysis needs enough of the owner's own sentences to hold
- VOICE-43 [o] 검증 checks that the voice applies: the owner picks one of the voice's answered prompts — or answers one there, the answer becoming a 학습 글 too — and one durable `check_voice` job on the account's active write-stage selection writes a short piece on that prompt's subject in this voice
  - the call receives the voice's projection (→VOICE-46) with that prompt's own answer withheld from the excerpts, the prompt, and a photo prompt's photo
  - the piece is `VOICE_CHECK_SENTENCES` (10~15) sentences with an opening and a closing ← two to five sentences cannot show a ratio
  - the result shows the piece beside the owner's answer with the fingerprint comparison (→VOICE-62)
  - a photo prompt is offered only while the write model reads images (`vision`), otherwise listed disabled with that reason
  - it needs a made voice; results stay on the 검증 tab newest first, one made before the current analysis marked so ← a paid result must not vanish
- VOICE-44 [o] a failed 검증 enqueue creates no result and blocks nothing; a failed 검증 shows its failure with a retry that starts a new job against the voice's current analysis
- VOICE-45 [o] voice jobs are ordinary queue work at boot (→GEN-33): an interrupted analysis or 검증 fails `JOB_INTERRUPTED` with its retry, a queued one runs, and no recovery repeats an uncertain paid call; page load, navigation, copy/export, polling, time and scheduler never enqueue or call a model for a voice; a failed job stays visible and retryable while the current analysis stays unchanged
- VOICE-46 [o] `PromptProfileForTopic(userID, voiceID, retrievalText, targetLanguage)` projects one made voice's current analysis:
  - a Korean target receives every counted item as plain Korean — what was counted and what its value means, never a bare key and number — then the AI part, then up to `VOICE_FEW_SHOT_MAX` (3) unique excerpts from the 학습 글 (topic matches first, stable recent fallback), each cut around `VOICE_FEW_SHOT_EXCERPT_TARGET_CHARS` (500) to at most `VOICE_FEW_SHOT_EXCERPT_MAX_CHARS` (800) characters
  - an English target receives the portable items alone (→LANG-15)
  - it never translates, falls back to another voice or reads another voice's 학습 글
  - a post with 말투 없음 gets no projection (→GEN-74)
- VOICE-47 [o] the projection tells the model to write in these habits without copying an excerpt's phrases or facts, and to follow the measured ending mix; zero excerpts is valid; no run of identical endings is voice text — it is a 기본 지침 (→GEN-75)
- VOICE-50 [o] every provider-backed operation freezes its voice before enqueue — generation and revision the owned post's voice or its absence, 말투 만들기 · 다시 분석 · 검증 the explicit voice, a 말투 반영 비교 its explicit voice (→MODEL-31), a retry the durable owner row's — never the post's current assignment
  - the handler rechecks that a frozen voice belongs to the account and never follows a later reassignment
  - `generation_jobs.voice_id` is empty for work with no voice
  - voice-owned kinds are guarded per `(voice_id, kind)` and a post-linked row must satisfy both the post guard and the voice guard
- VOICE-51 [o] no third-party sentence, Korean morphology, vector, embedding, SDK, Python, CGO, sidecar or model dependency: segmentation, counting, ending buckets, the readiness meter and excerpt retrieval use the Go standard library ← keeps the static SPA + distroless Go deployment and makes unit tests deterministic
- VOICE-52 [o] 말투 is the directory `/voices`, a LIST in 내 글's row shape (→POST-43): each active voice is one row and one link — its name, a `기본` badge on the 기본 and a meta line (`만드는 중 N%` until made, then the 학습 글 count and the analysis date) — with nothing interactive on the row ← one list design across the app
  - tombstones live in a `삭제된 말투 N개` disclosure rendered only when one exists, closed on load, its rows keeping `복원`
  - the page's one CTA is a docked `새 말투 만들기`, and an empty list says in plain words what a voice is
- VOICE-53 [o] `새 말투 만들기` — on the list and in a post's voice picker (→POST-101) — opens the shared `Sheet` (a bottom sheet below `md:`) with the name field and its `n / 50자` count
  - the commit action sits in flow after the field rather than in the pinned footer ← the panel is anchored to the layout viewport the software keyboard does not resize
  - success closes the sheet and opens the new voice on 학습 글
  - a refusal (a duplicate name) renders under the field in the user's words
- VOICE-54 [o] one voice is `/voices/$voiceId` with three sibling tabs sharing one row of links, the matching tab carrying `aria-current="page"`: 말투 분석 (the index) · `/materials` 학습 글 · `/checks` 검증
  - the title row names the voice and carries its rename (read-first behind a pencil; a tombstone stays renameable so a restore conflict can be resolved), `기본으로 설정` or `기본 해제` on a made voice, and `삭제`, confirmed through the sheet, which says what stays
  - a tombstone shows a notice with `복원` and blocks every change with the reason; an unowned id reads `없는 말투예요.`
  - `/voice` and `/voice/<tab>` redirect to the same tab of the 기본 voice, or to `/voices` when there is none, creating nothing
  - each tab issues only the queries its panel renders, and an in-flight list reads as loading rather than as an empty voice
  - the top-level destinations are 글 / 영상 / AI 모델, with 말투 inside the 글 group (→THEME-38)
- VOICE-55 [o] the paste form is disabled below 200 trimmed Unicode characters; 말투 만들기 and 다시 분석 are disabled below 100% or without a usable analyze selection, and 검증 without a usable write selection, each saying why in place; an RPC rejection renders from stable `AppFailure` reason/params in the active locale, never raw transport prose; `active_job_id` resumes polling and a finished job refreshes that voice's queries
- VOICE-56 [o] every voice query cache is partitioned by `(account, voice)` — `voices`, `voice-analysis`, `voice-materials`, `voice-checks` — so two voices of one account and two accounts on one device never read each other's entry
  - directory mutations patch the cached list in the server's order (create/rename/delete/restore upsert one voice, set-default installs the returned list)
  - rename, delete and restore also mark the voice's scope and every cached post stale
  - the SPA removes every account-scoped cache on logout and on a mid-session authentication failure
- VOICE-57 [x] a topic, project, category or folder hierarchy; a cross-voice 학습 글 picker or copy; moving 학습 글, analyses or 검증 results on reassignment; automatic voice selection from content; hard-deleting a voice; a voice-count cap; scheduled or automatic analysis, embeddings or a background judge; English voices, which would arrive with their own items and prompts — out of scope
- VOICE-58 [o] 학습 글 (their text and photos), analyses and 검증 results are private account data cascading from the owning account; a soft-deleted voice keeps every row and only stops accepting changes; no private prose or photo is logged; no cross-account or cross-voice retrieval, comparison, check or prompt input exists
- VOICE-59 [o] a voice is made only of 학습 글 the owner adds — posts they wrote by hand, pasted, and their answers to the product's prompts — and grows only the same two ways, then 다시 분석; never from a finished post, a revision, an edit diff or any AI output ← only the owner's own writing carries the owner's voice
- VOICE-60 [o] the prompts are one code-owned set shared by every voice, `VOICE_PROMPT_COUNT` (20): 4 openings, 12 descriptions (6 on a photo, 6 on a situation) and 4 closings, each asking for 2~5 sentences (`고른 사진을 블로그에 쓰듯 2~5문장으로 써 보세요`, `처음 가 본 곳에 들어섰을 때를 써 보세요`)
  - a photo prompt's photo is the owner's own, picked on their device, converted in the browser and stored privately as a post photo is (→POST-31 … POST-39); the product supplies no photo ← the owner writes about what they actually ate or saw, as when writing a post
  - the owner answers any prompt in any order and skips any; each prompt holds one answer, and deleting it frees the prompt
  - an answer is trimmed and non-empty; the prompt texts are product copy, never rows or config
- VOICE-61 [o] a line that is not the owner's prose counts toward no sentence and feeds no item: a hashtag-only line, a `[출처]` line, an info line (one that opens with 주소 · 영업시간 · 운영시간 · 전화 · 휴무 · 주차 · 가격 · 위치 followed by `:` or a space, or with 📍 ⏰ ☎️), a Korean address line, and a line holding no Hangul ← a voice is learned only from prose the owner wrote, and a pasted Naver post carries its place card and hashtags
- VOICE-62 [o] the fingerprint comparison measures a text by the analysis's own counting and shows each counted item as the voice's value beside the text's, in the item's own unit, the item farthest from the voice first (distance relative to the voice's own value); an item the text is too short to show reads 알 수 없음; it makes no call and is used by 검증 (→VOICE-43), ② (→POST-102) and 말투 반영 비교 (→MODEL-67)
- VOICE-63 [o] the 말투 분석 tab shows the current analysis read-only in two groups, `숫자로 본 습관` (each counted item as one plain sentence with its number, e.g. `문장의 32%를 느낌표로 끝내요`) and `AI가 읽은 인상` (the AI part), each item with its example sentence; no provenance badge and no per-item edit ← the group title says where a value came from, and the example lets the owner judge it at once
  - until the voice is made it shows the readiness meter and the way to 학습 글 in place of an analysis
  - VOICE-21's notice and `이전 분석으로 되돌리기` sit above the groups
- VOICE-64 [o] the 학습 글 tab lists the voice's 학습 글 newest first — a pasted post by its label, an answer by its prompt — each opening to its full text and photo with `삭제`; it carries `글 붙여넣기` (the paste form, 제목 (선택) first) and `문항 풀기` (the prompt list, answered ones marked), both taking entries until closed (→VOICE-65), and until the voice is made the readiness meter
- VOICE-65 [o] a 학습 글 sheet keeps taking entries until the owner closes it: a save never closes the sheet ← enough 학습 글 takes many entries, and reopening the sheet for each one breaks the run
  - 문항 풀기: a saved answer opens the next unanswered prompt in the set's order after it, wrapping to the start, on a blank form
  - `건너뛰기` opens that same next prompt without saving, offered only while another unanswered prompt exists; `문항 목록` returns to the list
  - once no unanswered prompt remains, the sheet shows the list with every prompt marked answered
  - 글 붙여넣기: a saved post empties the form for the next one, and its cancel action reads `닫기` from then on
  - each save is confirmed in place (`답을 저장했어요`, `글을 추가했어요`)
  - a refused save keeps the same entry with its text; closing the sheet discards only the unsaved entry
  - 검증's answer-one path still saves one answer and returns to 검증 (→VOICE-43)

## flow
- create: 새 말투 만들기 → CreateVoice(name) → `만드는 중` → 학습 글(글 붙여넣기 | 문항 풀기(photo on the owner's own photo | situation)) → next entry in the same sheet until it closes (→VOICE-65) → meter N% (no call) → 100% → 말투 만들기 → `analyze_voice`(count → one call) → made → 말투 분석
- grow: 학습 글 added | deleted → notice → 다시 분석(at 100%) → current = new, previous kept → 이전 분석으로 되돌리기
- check: 검증 → an answered prompt | answer one → `check_voice`(one write call, that answer withheld) → piece beside the answer + fingerprint comparison
- delete: DeleteVoice(busy → refuse | tombstone) → posts keep `VoiceRef{deleted}` → RestoreVoice | reassign the post

## constraints
- constants BE `internal/voice`: `VoiceNameMaxChars` 50 · `SampleMinChars` 200 · `LabelFallbackChars` 20 · `VOICE_READY_SENTENCES` 60 · `VOICE_PROMPT_COUNT` 20 (openings 4 · descriptions 12 · closings 4) · `VOICE_CHECK_SENTENCES` 10~15 · `VOICE_FEW_SHOT_MAX` 3 · `VOICE_FEW_SHOT_EXCERPT_TARGET_CHARS` / `MAX_CHARS` 500 / 800; FE constants live in their owning slices — `entities/voice/config` mirrors `VOICE_NAME_MAX_CHARS` 50 and the paste feature mirrors `VOICE_SAMPLE_MIN_CHARS` 200; the prompt set and the non-prose patterns are code; no env var, no schedule, no interval
- schema: `voices(id PK, user_id FK cascade, name, is_default, deleted_at, created_at, updated_at, UNIQUE(id, user_id))` with `voices_one_default` and `voices_active_name`, plus tables for 학습 글 (a pasted post or a prompt answer with its photo key), analyses (current and previous) and 검증 results, each keyed by `(user_id, voice_id)`
- placement BE: `backend/internal/voice` (domain, service, fingerprint counting, store, rpc, `schemas/`); it publishes ports consumed by post (`VoiceDirectory`), generation (`Profiles`) and experiment (`VoiceDirectory`, the fingerprint comparison), and asks `Jobs` before a delete; all adapted only in `cmd/api`; a boundary test forbids sibling `store`/`sqlc` imports inside `internal/`
- placement FE: `entities/voice` (model, api, ui, config) · `features/create-voice` `rename-voice` `set-default-voice` `delete-voice` `restore-voice` `select-post-voice` and the other verb slices taking `voiceId` · the fingerprint comparison widget shared by 검증, ② and the model lab · `pages/voices` `pages/voice` · `widgets/voice-warning` · `app/routes` (voice layout, `/voice` redirect)
- contracts: `voice.proto`, plus `VoiceRef` in `post.proto` and `voice_id` in `model_experiment.proto`

## chg
- r6 261001 VOICE-65+ a 학습 글 sheet keeps taking entries until closed: next unanswered prompt or 건너뛰기, a blank paste form, each save confirmed in place
