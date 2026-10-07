# VOICE voices
> r14 | Private personal/synthetic writing styles with editable revision-aware learning materials, accepted-profile reuse and explicit actual-writing test adoption.

## decisions
- VOICE-1 [o] account-owned writing voices are isolated and labelled personal or synthetic; each owns its materials and current/previous analysis. Personal voices learn only owner prose; generated styles and test winners follow VOICE-68/69 and are never personal-habit evidence.
- VOICE-2 [o] active display names are unique within the account (`voices_active_name`) and a tombstone's name reserves nothing; at most one active, made voice is the account's 기본 (partial unique index `voices_one_default`), and an account may have none
- VOICE-3 [o] every voice/material/analysis query and mutation is authenticated-owner and voice scoped; unknown/foreign IDs are indistinguishable. Tests freeze owned profiles under MODEL-30/88, never treating another voice's material as interchangeable.
- VOICE-4 [o] signup, provisioning, reads and page entry create no voice or analysis; a personal voice is created only on an explicit questionnaire/paste action and a generated style only on explicit adoption (→VOICE-68).
- VOICE-5 [o] a voice belongs to no 분야 and no template: it is the owner's own voice on a post of any 분야 and any template, and neither a template's text nor a guideline enters its analysis or changes it; no revision path writes voice state ← a fingerprint is surface habit, which holds across topics and forms, while the template decides the post's form (→TMPL-1)
- VOICE-6 [o] a personal analysis is the counted fingerprint of its owner materials plus the AI impression and cited examples; a synthetic analysis separately stores its origin and bounded illustrative sample under VOICE-69. Neither kind holds a user-authored rule override, and only one previous analysis is retained.
- VOICE-8 [o] a 학습 글 is private to its owner: its full text — and an answered photo prompt's photo — opens for the owner on the voice's 학습 데이터 screen (→VOICE-64) and reaches no other account, voice or log
- VOICE-9 [o] ListVoices returns the account's voices including tombstones — active first, the 기본 first among them, then by name and id — each carrying whether it is made and, until it is, its readiness (→VOICE-32); the order is part of the contract and the frontend re-applies it after a cache patch; GetVoiceProfile returns the voice summary with its current analysis
- VOICE-10 [o] CreateVoice trims the name, requires 1–`VoiceNameMaxChars` (50) Unicode scalar values (`InvalidArgument` otherwise), refuses an active-name collision (`AlreadyExists`) and creates a Korean voice that is not yet made (`만드는 중`, no analysis); it takes no language, description or 분야, calls no model, and there is no voice-count limit
- VOICE-12 [o] RenameVoice changes the display name of an active or deleted owned voice with the same validation, rewriting no post row and no frozen snapshot; SetDefaultVoice makes one active, made, owned voice the 기본 — atomically clearing the previous one — or clears the 기본 so the account has none, and answers with the whole directory ← the previous 기본 changed too
- VOICE-13 [o] DeleteVoice soft-deletes the voice and refuses an active analysis; deleting the default/last voice leaves none. Stored work, materials, analyses and paid test history remain private/readable. Tests keep frozen profiles but cannot directly use/adopt or newly start from a deleted voice; explicit saving of a previously tested synthetic winner as a new copy follows MODEL-90; tombstones leave selectable options and repeated deletion is a no-op.
- VOICE-14 [o] RestoreVoice clears `deleted_at` without enqueueing anything, fails `AlreadyExists` while an active voice holds the same name (rename the tombstone first), and never changes the default
- VOICE-15 [o] deleted voices remain readable but refuse material add/edit/delete, analysis/reanalysis, restore-analysis/default changes and new tests before jobs/provider work. Restoring and resolving active-name conflicts follow VOICE-14.
- VOICE-16 [o] reads, source edits, lifecycle changes, navigation, polling and export perform no model work. Provider work is limited to explicit analysis/reanalysis, requested generated-style preparation and unified actual-writing tests under MODEL.
- VOICE-20 [o] a pasted 학습 글 is trimmed and must contain at least `SampleMinChars` (200) Unicode characters (the rejection names the measured count); an empty label falls back to the first `LabelFallbackChars` (20) characters; pasting needs no model and enqueues nothing
- VOICE-21 [o] adding, editing or deleting a learning material performs no AI work.
  - unmade readiness uses current validated materials; a made voice keeps its accepted current analysis and accepted source revisions until successful explicit reanalysis
  - show pending material changes and explain that writing still uses the previous accepted analysis; offer an estimated explicit reanalysis action when eligible
  - new/revised prose cannot enter the accepted projection early; deleting a material withdraws its source excerpts/examples immediately while keeping the remaining accepted profile usable
  - title/label-only edits do not mark analysis dirty when the prose/photo semantics are unchanged
- VOICE-22 [o] analysis freezes exact owned material content revisions and necessary prose/photo semantics at admission and publishes that snapshot. Concurrent source edits remain marked pending after completion; retries and restore compare accepted revisions rather than material IDs alone. Reanalysis is never automatic.
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
- VOICE-26 [o] each immutable analysis privately records measured items, AI description/examples, accepted material IDs/content revisions and necessary accepted excerpt source text, count and time; retain only current and one previous analysis. Unknown facets remain unknown. Source deletion filters withdrawn excerpts/examples under VOICE-21.
- VOICE-27 [o] the product computes every counted item itself and never asks the model to estimate one; the call names every AI field it expects and attaches the embedded schema `internal/voice/schemas/voice_analysis.schema.json` when the resolved model declares `structured_output`, falling back to the prompt alone otherwise
- VOICE-30 [o] after a 다시 분석 the voice offers one `이전 분석으로 되돌리기`, which makes the previous analysis current again and discards the one it replaced; there is no redo and no older history, and a deleted voice offers none ← it exists for a re-analysis the owner dislikes
- VOICE-31 [o] GetVoiceProfile exposes active analysis job identity for durable polling/resume; unified tests use their own MODEL records and cannot masquerade as completed analysis.
- VOICE-32 [o] a personal voice becomes ready for its first explicit analysis after ten distinct valid question answers covering opening, description and closing, or the existing sixty owner-prose sentences covering those parts.
  - each question answer must contain owner-authored Korean prose; hashtags/info-only/emoji-only text does not advance readiness
  - the first ten-question set covers every part without a photo requirement; server readiness reports answered/required questions and sentence fallback separately
  - the percent is the larger valid answer-count/sentence share, held below 100 while a part is missing; short material preserves unknown fingerprint facets instead of inventing measured values
  - a personal voice remains unselectable and cannot be default before successful analysis; an adopted synthetic style is independently usable under VOICE-68
- VOICE-45 [o] analysis jobs follow GEN-33 boot recovery: interrupted calls fail with explicit retry, queued admitted work continues, and uncertain calls never repeat automatically. The last accepted profile survives failures. Historical admitted verification jobs settle safely under MODEL-76 without admitting new standalone checks.
- VOICE-46 [o] PromptProfileForTopic projects one owned made voice's accepted current analysis and its accepted material revision text.
  - Korean gets the counted explanations, AI description and up to VOICE_FEW_SHOT_MAX(3) unique topic-prioritized accepted excerpts, target500/max800 characters; English gets only LANG-15 portable items
  - current additions/edits do not enter excerpts before reanalysis; filter any source material that is no longer present/owned before retrieval
  - synthetic examples remain under VOICE-69, and no-voice posts carry no projection; never translate or fall back to another voice
- VOICE-47 [o] the projection tells the model to write in these habits without copying an excerpt's phrases or facts, and to follow the measured ending mix; zero excerpts is valid; no run of identical endings is voice text — it is a 기본 지침 (→GEN-75)
- VOICE-50 [o] provider work freezes the explicit owned voice/accepted profile before enqueue; analysis freezes owned material revisions, writing/revision freeze the assigned voice or absence, and tests freeze every selected entrant profile under MODEL-30. Retries use the admitted snapshot and never follow a later post assignment. Existing queue ownership guards still apply.
- VOICE-51 [o] no third-party sentence, Korean morphology, vector, embedding, SDK, Python, CGO, sidecar or model dependency: segmentation, counting, ending buckets, the readiness meter and excerpt retrieval use the Go standard library ← keeps the static SPA + distroless Go deployment and makes unit tests deterministic
- VOICE-52 [o] writing settings /voices list named owned styles with personal/synthetic provenance, default marker, learning/usable state, last analysis and pending-material/EDIT state.
  - use the shared settings-row language rather than inheriting operational post-history behavior
  - personal learning, explicit AI creation and unified actual-writing test entry are named distinct actions; tombstones remain in a closed restore disclosure
- VOICE-53 [o] `새 말투 만들기` — on the list and in a post's voice picker (→POST-101) — opens the shared `Sheet` (a bottom sheet below `md:`) with the name field and its `n / 50자` count
  - the commit action sits in flow after the field rather than in the pinned footer ← the panel is anchored to the layout viewport the software keyboard does not resize
  - success closes the sheet and opens the new voice on `/materials` 말투 학습
  - a refusal (a duplicate name) renders under the field in the user's words
- VOICE-54 [o] a made voice detail shows its name, settings hierarchy, analysis and learning materials, with a named common writing-test entry and relevant test history.
  - unmade voices open personal learning; legacy checks addresses show retained paid records/common test history and never start standalone verification
  - rename/default/delete/restore and legacy default-voice redirects retain their ownership/lifecycle rules
  - each panel reads only its own data; loading is distinct from empty, and location/return follow THEME-58
- VOICE-55 [o] pasted material creation/editing keeps the200-character rule; analysis/reanalysis needs actual server readiness and an eligible prepared analyze ref, while tests enforce MODEL eligibility. Refusals are localized stable failures, active jobs resume polling and completion refreshes the owned voice.
- VOICE-56 [o] every voice query cache is partitioned by `(account, voice)` — `voices`, `voice-analysis`, `voice-materials`, `voice-checks` — so two voices of one account and two accounts on one device never read each other's entry
  - directory mutations patch the cached list in the server's order (create/rename/delete/restore upsert one voice, set-default installs the returned list)
  - rename, delete and restore also mark the voice's scope and every cached post stale
  - the SPA removes every account-scoped cache on logout and on a mid-session authentication failure
- VOICE-57 [x] a topic, project, category or folder hierarchy; a cross-voice 학습 글 picker or copy; moving 학습 글, analyses or 검증 results on reassignment; automatic voice selection from content; hard-deleting a voice; a voice-count cap; scheduled or automatic analysis, embeddings or a background judge; English voices, which would arrive with their own items and prompts — out of scope
- VOICE-58 [o] materials, accepted source revisions, analyses and test/history references are private owner data; account deletion cascades all of them. Soft deletion preserves history but refuses changes. No prose/photo is logged and no cross-owner or cross-voice retrieval or test source exists.
- VOICE-59 [o] personal learning contains only owner-authored pasted writing and prompt answers, never generated posts, revisions, edit diffs or AI candidates. Synthetic generation/adoption is a separate explicitly labelled origin under VOICE-68/69, with no synthetic example inserted into personal materials or readiness.
- VOICE-60 [o] writing questions form an extensible code-owned catalog of at least 200 stable keys with natural Korean situational wording, a concrete scene, optional writing hint, part and starter designation.
  - preserve all existing prompt keys and saved-answer/photo semantics; improve their wording without invalidating answers
  - the starter set has ten different ordinary-life situations covering opening/description/closing, requires no photo upload and asks for one to three natural sentences in the owner's usual style
  - further questions cover varied everyday feelings, recommendations, surprises, frustrations, decisions, places and objects; no prompt requires an invented personal history or specialist knowledge
  - optional photo questions still use only owner-picked private photos; answers are nonempty Korean prose, saving again replaces only that key
  - reads and question browsing create no provider work; text and catalog keys are code, not generated at runtime
- VOICE-61 [o] a line that is not the owner's prose counts toward no sentence and feeds no item: a hashtag-only line, a `[출처]` line, an info line (one that opens with 주소 · 영업시간 · 운영시간 · 전화 · 휴무 · 주차 · 가격 · 위치 followed by `:` or a space, or with 📍 ⏰ ☎️), a Korean address line, and a line holding no Hangul ← a voice is learned only from prose the owner wrote, and a pasted Naver post carries its place card and hashtags
- VOICE-62 [o] pure local fingerprint comparison remains an optional diagnostic beside actual test writings or POST-102 editing: show measurable profile/text values in their units and unknown when unavailable. It makes no AI call and is not a separate verification product or an automatic judge.
- VOICE-63 [o] the 말투 분석 tab shows the current analysis read-only: each counted item as one plain sentence with its number (e.g. `문장의 32%를 느낌표로 끝내요`) without a `숫자로 본 습관` group title, followed by `AI가 읽은 인상`; each item has its example sentence, with no provenance badge or per-item edit
  - VOICE-21's notice and `이전 분석으로 되돌리기` sit above the groups
- VOICE-64 [o] learning materials list newest first and open to their full text/photo with explicit Edit, Save/Cancel and Delete.
  - pasted material editing keeps VOICE-20 rules; saved prompt-answer editing retains its stable question identity and applicable body/photo rules
  - use expected content revision; a conflict preserves unsaved input with a recovery action
  - materials, analysis and directory expose VOICE-21 pending-change state and the prior-profile explanation; analysis stays an explicit estimated action
- VOICE-65 [o] questionnaire sessions present ten questions at a time, one concrete scene and answer per view, with saved-count progress, Back and an explicit skip that replaces a skipped question rather than counting it as answered.
  - saved answers and stable keys are the resume authority; failed saves retain text/photo and the same question, and duplicate/stale submit responses cannot advance another owner or question
  - the first session uses the starter questions and stops after ten valid saved answers with an explicit analysis action once ready; it never requires exhausting the catalog or padding every answer to many sentences
  - further sessions choose ten unanswered questions, preserving all previous answers; the catalog can keep growing into hundreds without changing the saved format
  - editing a saved answer is explicit; question exhaustion offers review/completion instead of silently rewriting earlier answers
  - questionnaire UI is reusable inline during setup and in settings; pasted material and explicit saved-answer editing retain their source validation/photo contracts
- VOICE-66 [o] writing-voice setup starts with three understandable peer methods: use owner-written text, answer ten everyday situations, or choose from eight explicitly AI-generated styles.
  - opening or inspecting methods creates no voice, analysis or provider work; an explicit personal-learning action may create/resume a named personal voice
  - each method opens its own input/review flow; questionnaire wording does not ask for an existing written post, and pasting does not expose unanswered questions
  - existing work offers an explicit continuation into its actual current stage; synthetic examples remain separate from personal writing
- VOICE-67 [o] personal questionnaire readiness is not a promise that every fingerprint facet is known; the first profile may be used with measured/unknown facets, and additional personal answers improve the next explicitly requested analysis.
- VOICE-68 [o] explicit writing-style preparation creates a durable owner batch with requested2/4/8/16 distinct candidates; ordinary recommendation defaults to eight and test preparation uses the selected format count.
  - explain synthetic provenance, show an estimate and admit one bounded prepared write call with count-specific budgets/schema
  - each validated candidate has a name, readable character and labelled fictional example/projection; incomplete/duplicate/invalid batches fail, never silently shrink or retry
  - one ordinary candidate batch may be active per account; failed/cancelled/incomplete batches cannot be adopted
  - ordinary adoption is owner/job/candidate-idempotent, uses bounded unique name suffixes, and applies an optional default only at its first confirmed publication
  - candidates are private unsaved drafts; choosing one performs no AI, and explicit ordinary adoption or test-winner saving alone creates a usable owned style
  - closed/reopened work resumes its actual durable state without a new call; abandoned work settles confirmed usage under QUOTA
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

- VOICE-72 [o] content revision and accepted material version identify analysis freshness; metadata-only edits do not affect it. Current/previous profile restoration and jobs compare full source versions, not just the set of material IDs.
- VOICE-73 [o] unified actual-writing tests compare owned made personal/synthetic voices or validated generated drafts under MODEL; generated champion adoption preserves its tested synthetic profile and never inserts AI posts/examples into personal readiness or materials.
- VOICE-74 [o] an analysis without provable accepted source revisions retains its stored usable profile with unknown source freshness; never reconstruct historical excerpts from current edited material or claim verified freshness. Explicit reanalysis establishes a new accepted snapshot, while existing deletion filters still withdraw known source examples.

## flow
- personal start: choose ten questions | paste own writing → explicitly create/resume one personal voice → save owner answers in a ten-question session → ten valid answers with every part or sixty owner sentences → explicit analysis → made personal voice → optional default → creation
- further learning: choose another ten unanswered situations | paste → save private materials → keep existing analysis → explicit reanalysis → current personal snapshot with previous retained
- generated styles: explicit generation with visible estimate → bounded durable one-call batch → requested2/4/8/16 labelled styles (eight by default) → explicit selection/adoption → atomic made synthetic voice/default choice → creation
- questionnaire: starter/unanswered batch → scene + natural answer → guarded save → next question | failure retains answer → ten answers → explicit analysis or completion; saved-key resume after navigation/reload
- test: select one variable and2/4/8/16 entrants → common real-writing material → explicit generation → human knockout decisions → optional champion adoption
- delete: guarded soft delete → retained historical refs → explicit restore/reassignment

## constraints
- constants BE `internal/voice`: `VoiceNameMaxChars` 50 · `SampleMinChars` 200 · `LabelFallbackChars` 20 · `VOICE_READY_SENTENCES` 60 · `VOICE_INITIAL_QUESTION_COUNT` 10 · code-owned prompt catalog ≥200 · `VOICE_CANDIDATE_COUNT` default8, allowed2/4/8/16 · `VOICE_FEW_SHOT_MAX` 3 · `VOICE_FEW_SHOT_EXCERPT_TARGET_CHARS` / `MAX_CHARS` 500 / 800; FE constants live in their owning slices — `entities/voice/config` mirrors `VOICE_NAME_MAX_CHARS` 50 and the paste feature mirrors `VOICE_SAMPLE_MIN_CHARS` 200; the prompt set and the non-prose patterns are code; no env var, no schedule, no interval
- schema: `voices(id PK, user_id FK cascade, name, is_default, deleted_at, created_at, updated_at, UNIQUE(id, user_id))` with `voices_one_default` and `voices_active_name`, plus tables for 학습 글 (a pasted post or a prompt answer with its photo key), analyses (current and previous) and read-only paid check history, each keyed by `(user_id, voice_id)`
- placement BE: `backend/internal/voice` (domain, service, fingerprint counting, store, rpc, `schemas/`); it publishes ports consumed by post (`VoiceDirectory`), generation (`Profiles`) and experiment (`VoiceDirectory`, the fingerprint comparison), and asks `Jobs` before a delete; all adapted only in `cmd/api`; a boundary test forbids sibling `store`/`sqlc` imports inside `internal/`
- placement FE: `entities/voice` (model, api, ui, config) · `features/create-voice` `rename-voice` `set-default-voice` `delete-voice` `restore-voice` `select-post-voice` and the other verb slices taking `voiceId` · the pure fingerprint diagnostic shared by writing/tests and retained paid history · `pages/voices` `pages/voice` · `widgets/voice-warning` · `app/routes` (voice layout, `/voice` redirect)
- contracts: `voice.proto`, plus `VoiceRef` in `post.proto` and `voice_id` in `model_experiment.proto`

## chg
-
