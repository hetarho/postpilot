# VOICE voices
> r13 | Mutually isolated personal writing voices learned from ten-question sessions or owner writing, and explicitly generated eight-style batches with labelled synthetic provenance.

## decisions
- VOICE-1 [o] account-owned writing voices are mutually isolated and have explicit personal or synthetic provenance; each owns its materials, current/previous analysis and verification results under (user_id, voice_id). Personal voices learn only owner-authored writing; synthetic styles follow VOICE-68/69 and never become evidence of the owner's personal habits.
- VOICE-2 [o] active display names are unique within the account (`voices_active_name`) and a tombstone's name reserves nothing; at most one active, made voice is the account's 기본 (partial unique index `voices_one_default`), and an account may have none
- VOICE-3 [o] every store query and RPC is scoped by the authenticated user and by one voice, named explicitly (`voice_id` on analysis, 학습 글, 검증 and comparison requests) or derived from an owned aggregate (a post); no request carries a user id; a foreign voice id is indistinguishable from an unknown one (`NotFound`); a same-account id from another voice is refused — never interchangeable material
- VOICE-4 [o] signup, provisioning, reads and page entry create no voice or analysis; a personal voice is created only on an explicit questionnaire/paste action and a generated style only on explicit adoption (→VOICE-68).
- VOICE-5 [o] a voice belongs to no 분야 and no template: it is the owner's own voice on a post of any 분야 and any template, and neither a template's text nor a guideline enters its analysis or changes it; no revision path writes voice state ← a fingerprint is surface habit, which holds across topics and forms, while the template decides the post's form (→TMPL-1)
- VOICE-6 [o] a personal analysis is the counted fingerprint of its owner materials plus the AI impression and cited examples; a synthetic analysis separately stores its origin and bounded illustrative sample under VOICE-69. Neither kind holds a user-authored rule override, and only one previous analysis is retained.
- VOICE-8 [o] a 학습 글 is private to its owner: its full text — and an answered photo prompt's photo — opens for the owner on the voice's 학습 데이터 screen (→VOICE-64) and reaches no other account, voice or log
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
- VOICE-16 [o] reads, lifecycle edits, gathering, page entry, polling, copying/export and boot issue no model call or job. Provider work is limited to explicit analysis/reanalysis, verification, model comparison under MODEL-67 and generated-style requests under VOICE-68.
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
- VOICE-32 [o] a personal voice becomes ready for its first explicit analysis after ten distinct valid question answers covering opening, description and closing, or the existing sixty owner-prose sentences covering those parts.
  - each question answer must contain owner-authored Korean prose; hashtags/info-only/emoji-only text does not advance readiness
  - the first ten-question set covers every part without a photo requirement; server readiness reports answered/required questions and sentence fallback separately
  - the percent is the larger valid answer-count/sentence share, held below 100 while a part is missing; short material preserves unknown fingerprint facets instead of inventing measured values
  - a personal voice remains unselectable and cannot be default before successful analysis; an adopted synthetic style is independently usable under VOICE-68
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
  - docked actions offer `새 말투 만들기` for personal learning and `AI 말투 추천받기` for explicit generated styles; an empty list explains personal learning in plain words
- VOICE-53 [o] `새 말투 만들기` — on the list and in a post's voice picker (→POST-101) — opens the shared `Sheet` (a bottom sheet below `md:`) with the name field and its `n / 50자` count
  - the commit action sits in flow after the field rather than in the pinned footer ← the panel is anchored to the layout viewport the software keyboard does not resize
  - success closes the sheet and opens the new voice on `/materials` 말투 학습
  - a refusal (a duplicate name) renders under the field in the user's words
- VOICE-54 [o] one made voice is `/voices/$voiceId` with three sibling tabs sharing one row of links, the matching tab carrying `aria-current="page"`: 말투 분석 (the index) · `/materials` 학습 데이터 · `/checks` 검증
  - until its first analysis succeeds, the voice shows only the `/materials` 말투 학습 screen with no tab row; the index and `/checks` addresses lead to that screen
  - the title row names the voice and carries its rename (read-first behind a pencil; a tombstone stays renameable so a restore conflict can be resolved), `기본으로 설정` or `기본 해제` on a made voice, and `삭제`, confirmed through the sheet, which says what stays
  - a tombstone shows a notice with `복원` and blocks every change with the reason; an unowned id reads `없는 말투예요.`
  - `/voice` and `/voice/<tab>` redirect to the same tab of the 기본 voice, or to `/voices` when there is none, creating nothing
  - each tab issues only the queries its panel renders, and an in-flight list reads as loading rather than as an empty voice
  - 말투 is a writing settings destination under THEME-38, with optional first-use learning under AUTH-51
- VOICE-55 [o] the paste form retains its 200-character rule; analysis/reanalysis require server readiness and an eligible prepared analyze ref, verification an eligible write ref. Initial model defaults follow MODEL-87 without a model-selection screen. Refusals use localized stable AppFailure messages and active jobs resume polling; completion refreshes the owning voice.
- VOICE-56 [o] every voice query cache is partitioned by `(account, voice)` — `voices`, `voice-analysis`, `voice-materials`, `voice-checks` — so two voices of one account and two accounts on one device never read each other's entry
  - directory mutations patch the cached list in the server's order (create/rename/delete/restore upsert one voice, set-default installs the returned list)
  - rename, delete and restore also mark the voice's scope and every cached post stale
  - the SPA removes every account-scoped cache on logout and on a mid-session authentication failure
- VOICE-57 [x] a topic, project, category or folder hierarchy; a cross-voice 학습 글 picker or copy; moving 학습 글, analyses or 검증 results on reassignment; automatic voice selection from content; hard-deleting a voice; a voice-count cap; scheduled or automatic analysis, embeddings or a background judge; English voices, which would arrive with their own items and prompts — out of scope
- VOICE-58 [o] 학습 글 (their text and photos), analyses and 검증 results are private account data cascading from the owning account; a soft-deleted voice keeps every row and only stops accepting changes; no private prose or photo is logged; no cross-account or cross-voice retrieval, comparison, check or prompt input exists
- VOICE-59 [o] personal learning contains only owner-authored pasted writing and prompt answers, never generated posts, revisions, edit diffs or AI candidates. Synthetic generation/adoption is a separate explicitly labelled origin under VOICE-68/69, with no synthetic example inserted into personal materials or readiness.
- VOICE-60 [o] writing questions form an extensible code-owned catalog of at least 200 stable keys with natural Korean situational wording, a concrete scene, optional writing hint, part and starter designation.
  - preserve all existing prompt keys and saved-answer/photo semantics; improve their wording without invalidating answers
  - the starter set has ten different ordinary-life situations covering opening/description/closing, requires no photo upload and asks for one to three natural sentences in the owner's usual style
  - further questions cover varied everyday feelings, recommendations, surprises, frustrations, decisions, places and objects; no prompt requires an invented personal history or specialist knowledge
  - optional photo questions still use only owner-picked private photos; answers are nonempty Korean prose, saving again replaces only that key
  - reads and question browsing create no provider work; text and catalog keys are code, not generated at runtime
- VOICE-61 [o] a line that is not the owner's prose counts toward no sentence and feeds no item: a hashtag-only line, a `[출처]` line, an info line (one that opens with 주소 · 영업시간 · 운영시간 · 전화 · 휴무 · 주차 · 가격 · 위치 followed by `:` or a space, or with 📍 ⏰ ☎️), a Korean address line, and a line holding no Hangul ← a voice is learned only from prose the owner wrote, and a pasted Naver post carries its place card and hashtags
- VOICE-62 [o] the fingerprint comparison measures a text by the analysis's own counting and shows each counted item as the voice's value beside the text's, in the item's own unit, the item farthest from the voice first (distance relative to the voice's own value); an item the text is too short to show reads 알 수 없음; it makes no call and is used by 검증 (→VOICE-43), ② (→POST-102) and 말투 반영 비교 (→MODEL-67)
- VOICE-63 [o] the 말투 분석 tab shows the current analysis read-only: each counted item as one plain sentence with its number (e.g. `문장의 32%를 느낌표로 끝내요`) without a `숫자로 본 습관` group title, followed by `AI가 읽은 인상`; each item has its example sentence, with no provenance badge or per-item edit
  - VOICE-21's notice and `이전 분석으로 되돌리기` sit above the groups
- VOICE-64 [o] the 학습 데이터 tab lists the voice's 학습 글 newest first — a pasted post by its label, an answer by its prompt — each opening to its full text and photo with `삭제`; it carries `글 붙여넣기` (the paste form, 제목 (선택) first) and `문항 풀기` (the sequential prompt sheet →VOICE-65), both taking entries until closed, and until the voice is made the readiness meter
- VOICE-65 [o] questionnaire sessions present ten questions at a time, one concrete scene and answer per view, with saved-count progress, Back and an explicit skip that replaces a skipped question rather than counting it as answered.
  - saved answers and stable keys are the resume authority; failed saves retain text/photo and the same question, and duplicate/stale submit responses cannot advance another owner or question
  - the first session uses the starter questions and stops after ten valid saved answers with an explicit analysis action once ready; it never requires exhausting the catalog or padding every answer to many sentences
  - further sessions choose ten unanswered questions, preserving all previous answers; the catalog can keep growing into hundreds without changing the saved format
  - editing a saved answer is explicit; question exhaustion offers review/completion instead of silently rewriting earlier answers
  - questionnaire UI is reusable inline during setup and in settings; pasted writing and verification's answer-one path retain their separate save contracts
- VOICE-66 [o] writing-voice setup starts with three understandable peer methods: use owner-written text, answer ten everyday situations, or choose from eight explicitly AI-generated styles.
  - opening or inspecting methods creates no voice, analysis or provider work; an explicit personal-learning action may create/resume a named personal voice
  - each method opens its own input/review flow; questionnaire wording does not ask for an existing written post, and pasting does not expose unanswered questions
  - existing work offers an explicit continuation into its actual current stage; synthetic examples remain separate from personal writing
- VOICE-67 [o] personal questionnaire readiness is not a promise that every fingerprint facet is known; the first profile may be used with measured/unknown facets, and additional personal answers improve the next explicitly requested analysis.
- VOICE-68 [o] an owner-triggered writing-style generation is one durable account-owned batch with exactly eight distinct candidates presented together.
  - the same explicit generate/compare/adopt flow is available during initial setup and later from `/voices` writing settings; opening or closing its wide sheet starts no AI, closing preserves durable work, and successful adoption opens the confirmed voice
  - one eligible saved write-model ref, randomized contrasting style directions, a shared fictional preview situation and bounded output are frozen at admission; one planned metered completion call produces the batch, without automatic paid corrections or fallback
  - candidates contain a short name, plain-language feel and a readable Korean sample; generated text is labelled AI-created style, not inferred personal identity or real owner facts
  - show the bounded credit estimate before explicit generation, reserve through QUOTA-13/14 and settle confirmed usage under QUOTA-46/49/52; regeneration requires another explicit request
  - one active batch per account, durable owner-only polling/result reads, explicit cancellation and no adoption of incomplete/failed/cancelled output; restart never repeats uncertain provider work
  - explicit adoption atomically creates one made voice, its synthetic current analysis and an idempotency record for (owner, job, candidate); it performs no AI call and creates no personal sample
  - duplicate adoption returns the same voice without undoing subsequent explicit changes; display-name collisions receive a bounded unique suffix, and optional defaulting applies only on the first adoption
- VOICE-69 [o] synthetic provenance is preserved in saved analysis and exposed wherever a style is chosen or inspected.
  - count the synthetic example separately, describe its requested style, and keep its sample labelled fictional; do not claim its ratios were measured from the owner's writing
  - synthetic samples are absent from personal readiness, sample CRUD and personal excerpt retrieval; style projection may use the bounded labelled synthetic example while explicitly forbidding copied facts or phrases
  - personal material can accumulate on an adopted style; successful explicit personal reanalysis changes the current origin to personal and retains the prior synthetic snapshot under the existing restore contract
  - snapshots without an origin decode as personal, preserving existing analyses and posts
- VOICE-70 [o] generated writing styles may be selected and refined conversationally under EDIT before explicit publication. Synthetic provenance and personal-material isolation remain mandatory; personal voice refinement publishes a new synthetic style, and existing synthetic refinement also publishes a new labelled style without replacing the source analysis.
- VOICE-71 [o] an unmade personal voice uses a guided learning sequence: choose paste or ten questions, save private material, review confirmed readiness, explicitly analyze, then inspect/use the made voice.
  - analysis is absent until actual readiness allows it; opening, saving a sample, question navigation and review never start analysis automatically
  - collected material is reviewable through a secondary action, not a permanently expanded sample list under the initial form
  - additional paste, another question batch and answer edits are optional explicit paths after the focused first batch; domain readiness and existing analysis remain authoritative
  - closing and returning recover confirmed samples and active analysis without replaying create/analyze/default actions

## flow
- personal start: choose ten questions | paste own writing → explicitly create/resume one personal voice → save owner answers in a ten-question session → ten valid answers with every part or sixty owner sentences → explicit analysis → made personal voice → optional default → creation
- further learning: choose another ten unanswered situations | paste → save private materials → keep existing analysis → explicit reanalysis → current personal snapshot with previous retained
- generated styles: explicit generation with visible estimate → bounded durable one-call batch → eight labelled styles → explicit selection/adoption → atomic made synthetic voice/default choice → creation
- questionnaire: starter/unanswered batch → scene + natural answer → guarded save → next question | failure retains answer → ten answers → explicit analysis or completion; saved-key resume after navigation/reload
- check: answered prompt or answer one → explicit check job → withheld-answer comparison
- delete: guarded soft delete → retained historical refs → explicit restore/reassignment

## constraints
- constants BE `internal/voice`: `VoiceNameMaxChars` 50 · `SampleMinChars` 200 · `LabelFallbackChars` 20 · `VOICE_READY_SENTENCES` 60 · `VOICE_INITIAL_QUESTION_COUNT` 10 · code-owned prompt catalog ≥200 · `VOICE_CANDIDATE_COUNT` 8 · `VOICE_CHECK_SENTENCES` 10~15 · `VOICE_FEW_SHOT_MAX` 3 · `VOICE_FEW_SHOT_EXCERPT_TARGET_CHARS` / `MAX_CHARS` 500 / 800; FE constants live in their owning slices — `entities/voice/config` mirrors `VOICE_NAME_MAX_CHARS` 50 and the paste feature mirrors `VOICE_SAMPLE_MIN_CHARS` 200; the prompt set and the non-prose patterns are code; no env var, no schedule, no interval
- schema: `voices(id PK, user_id FK cascade, name, is_default, deleted_at, created_at, updated_at, UNIQUE(id, user_id))` with `voices_one_default` and `voices_active_name`, plus tables for 학습 글 (a pasted post or a prompt answer with its photo key), analyses (current and previous) and 검증 results, each keyed by `(user_id, voice_id)`
- placement BE: `backend/internal/voice` (domain, service, fingerprint counting, store, rpc, `schemas/`); it publishes ports consumed by post (`VoiceDirectory`), generation (`Profiles`) and experiment (`VoiceDirectory`, the fingerprint comparison), and asks `Jobs` before a delete; all adapted only in `cmd/api`; a boundary test forbids sibling `store`/`sqlc` imports inside `internal/`
- placement FE: `entities/voice` (model, api, ui, config) · `features/create-voice` `rename-voice` `set-default-voice` `delete-voice` `restore-voice` `select-post-voice` and the other verb slices taking `voiceId` · the fingerprint comparison widget shared by 검증, ② and the model lab · `pages/voices` `pages/voice` · `widgets/voice-warning` · `app/routes` (voice layout, `/voice` redirect)
- contracts: `voice.proto`, plus `VoiceRef` in `post.proto` and `voice_id` in `model_experiment.proto`

## chg
-
