# GEN generation, revision, job queue
> r26 | Semantic-origin-aware writing and revision with grounded tags up to an upper bound, explicit evidence contracts and durable bounded jobs.

## decisions
- GEN-1 [o] a generated post is a `PostContent` block array, never HTML (I2); the flat `Block` types `TEXT` `HEADING` `IMAGE` `GALLERY` `VIDEO` `QUOTE` `LIST` are the model-facing protojson contract (`GALLERY` is a photo group carrying its photos' filenames in order, a layout, one `alt` and one `caption` →GEN-77; `VIDEO` carries the IMAGE fields →VIDEO-2)
- GEN-2 [o] every model-produced block passes one validation function right after parsing: missing required fields, fields forbidden for the type, empty list items and unknown types drop that block and log its type and field; an invalid heading level is clamped to 2; a non-heading `level` is normalized to 0 silently ← the structured-output schema requires `level` on every block; then every `IMAGE.file` is matched exactly and case-sensitively against the attached photo filenames and every `VIDEO.file` against the attached video filenames, and the rest are dropped and logged (→VIDEO-12)
- GEN-4 [o] ordinary generation and explicit compatible model-test output application replace canonical content wholesale, establish the machine baseline and move to review. Test generation and match decisions never mutate a source; no implicit content application follows a champion.
- GEN-5 [o] ordinary generation is a durable `generate` job:
  - StartGeneration validates ownership and one explicit active write model (plus an explicit vision-capable observe model when photos or videos exist, one carrying `video_input` when videos exist →VIDEO-11)
  - it freezes the required target language, the optional target length, the tag count, the post's optional template brief, the guideline texts (→GEN-15), the stored storyline when the run writes from it (→GEN-70), the ticked quality-rule texts (→GEN-51), whether the write model takes its effort natively (`write_native_effort` →MODEL-46) and — only when the post opted in — the retrieved memory texts into the payload (→MEM-19)
  - it returns only `job_id` — no provider call, no experiment
- GEN-6 [o] explicit unified writing tests freeze one MODEL-30 factor and exactly2/4/8/16 validated entrants before a durable test/job is admitted; candidate preparation is separate from actual-writing generation. Ordinary editor writing remains a single-writer operation.
- GEN-7 [o] with photos, observation always precedes writing: photos ordered by `created_at, id`, read from private storage as the already-normalized JPEGs and capped again at `MaxImageBytes` by response length and by the stream, sent in batches of `OBSERVE_BATCH_SIZE` (4); each attached video follows in its own call, delivered by reference (→VIDEO-8 →VIDEO-10); the server decodes no image and reads no video
- GEN-8 [o] a run observes only the photos it was frozen to observe and reuses the rest: a post with photos and at least one reusable stored observation opens the re-observation picker first, a post with nothing reusable starts directly
  - every checkbox starts CLEAR so confirming untouched reuses everything and skips observation ← the common case is a write stage that failed on photos that did not change
  - a photo with nothing to reuse (no entry, or none of scene, mood, visible text, objects, people) is checked, cannot be cleared, and is forced into the selection server-side ← a run must not write from a photo it has never looked at
  - a changed observation model warns naming both models but does not compel
- GEN-9 [o] the selection is frozen at enqueue with presence: a `ReobserveSelection` message absent means observe every attached photo (a client predating the picker), present with an empty list means observe nothing; the server intersects it with the attached photos, drops unknown names, forces the non-reusable ones, and freezes the resolved filenames together with the whole reusable snapshot into the payload; editing photos or switching models afterwards cannot change queued work; shared-observation tests retain the same explicit reuse selection; observer-factor tests independently observe their frozen material
- GEN-10 [o] each observation batch is matched back by exact filename; results for unknown filenames are discarded; a missing result becomes an empty observation; every entry records the model ref that produced it, per entry ← re-observing a subset is what lets one snapshot hold two models' work; an entry written before provenance existed reads as unknown
- GEN-11 [o] every persist writes the merged snapshot and it never shrinks: the freeze-time seed with this run's fresh entries laid over it, so a partial re-observation is complete from the first persist; a photo neither half covers is left out rather than given an empty entry (an unseeded full run's persist sequence is unchanged); an empty frozen set makes no provider call and no snapshot write; observation progress totals the frozen set, so a reuse-everything run reports `observe 0/0` and proceeds to writing at once
- GEN-12 [o] the writing stage is shown only the photos the run has eyesight for: attachments are read live at dequeue and narrowed to the merged snapshot, so a photo confirmed between enqueue and dequeue is neither observed nor named and belongs to the next run; a deleted photo's observation is deleted with it (→POST-18)
- GEN-13 [o] with no photos and no videos, observation makes no provider call, reports `observe 0/0`, clears stale observations and tells the writing model to use the memo without images
- GEN-14 [o] write prompt order:
  - stable request contracts: the flat GEN-1 answer shape, direct-path storyline first (→GEN-67), one paragraph per TEXT block, exact attached filenames, one placement per attachment and GEN-77 photo-group shape, template photo placement by section meaning (→TMPL-21), direct-path attachment coverage or frozen-story coverage (→GEN-70), one-line summary, at most the frozen `tag_count` tags, and the frozen output language
  - → frozen accepted voice projection or no voice bytes (→VOICE-46 →GEN-74)
  - → optional frozen template brief, preserving literal/instruction/fact boundaries (→TMPL-70)
  - → one stage-applicable `[작문 지침]` section (→GUIDE-15)
  - → optional ticked quality rules (→GEN-51)
  - → per-post title hint, memo, optional frozen memories, observations with filenames/orientation, and optional frozen storyline
  - material interpretation and honest semantic origins are stable contracts (→GEN-80–84); writing preferences, specific tag selection and naturalness remain switchable 기본 지침 ← interpreting evidence must stay reliable while owners retain control of writing direction

- GEN-15 [o] the template brief, the guideline texts — the enabled 기본 지침 in the product's order, then the applicable 지침 (→GUIDE-14) — and the selected memories are resolved once at enqueue from the post's template id through the template and guideline contexts' published lookups and written into the payload as text
  - the brief carrying the post's answers to the template's data fields already substituted and every field switched off or left blank already dropped (→TMPL-45), so an off field leaves the payload byte-identical to a template that never carried it and no separate answer field rides along
  - handlers build the prompt from the payload and never re-read the rows, so editing or deleting them later — across a restart-resume or an explicit retry — cannot change queued work
  - a template deleted before the start is simply absent
  - with no enabled 기본 지침 and no applicable 지침 there is no section
  - the observe stage never receives either
- GEN-16 [o] grounding is the first 기본 지침 of the 지침 kind (→GUIDE-41): while enabled, concrete facts must come from memo, observations, explicit template answers, opted-in memories or supported storyline material; a data-field value supplies only its own fact (→TMPL-46).
  - invent no interaction, facility, service, conversation, price or objective experience; omit unsupported facts or stay within the observed range
  - the impression guideline preserves the owner's stated taste/verdict and permits visibly identified AI expression proposals under GEN-83; preference-derived impressions additionally require GEN-73
  - disabling either recommendation removes that writing restriction, while honest origins and the photo chronology contract remain under GEN-80–84; observing receives neither preference guideline

- GEN-17 [o] the naturalness baseline is the last 기본 지침 of the 지침 kind (→GUIDE-41), offered and injected for a Korean target alone and absent for an English one, inside the write and revise prompts' `[작문 지침]` section
  - it is subtraction-only — capping stock antithesis, formulaic closers, obligation-ended paragraphs, connective-ending commas, uniform sentence structure, generic policy verbs, piled invented metaphors, abstract-noun chains, hype and unwarranted rhetoric — on newly written or explicitly revised TEXT prose only (titles, summaries, HEADING/LIST content and untouched text excluded)
  - it adds no extra pass or call
  - the voice's analysis outranks it on conflict
  - it must never gain bans on `~에 대해`, `~를 통해`, `~것이다` or sentence-initial conjunctions ← those folk rules were rejected against the corpus
- GEN-18 [o] test snapshots freeze common material/options and each server-resolved variant under MODEL-30/88. Apart from the chosen factor, prepared writer inputs are byte-identical; observer tests may differ in observations derived from the chosen observer. The hash covers common inputs, all variant revisions and prompt/schema versions; explicit names identify what is tested.
- GEN-19 [o] ordinary generation prepares observations once and persists one validated writer output. Tests store one validated complete post per entrant privately; share observation preparation unless the observe model is the selected factor, in which case each observer's result feeds the same fixed writer. Prepared observations/output never mutate the source before an explicit compatible MODEL-36 action.
- GEN-20 [o] a model declaring structured output receives the relevant JSON schema; other models use the same parser, which accepts direct JSON, fenced JSON or the first complete JSON object; unparseable output is the durable reason `MODEL_OUTPUT_INVALID` with the raw head (`BadOutputErrorHeadChars` 200) as diagnostic detail only; output with terminal reason `length` and no usable content, including partial JSON, is budget exhaustion `MODEL_OUTPUT_TRUNCATED` with a shorter-target or different-model remedy; a failed write or observe candidate still returns provider-reported usage; revision uses the same classification
- GEN-21 [o] every provider call is bounded by `LLMStageTimeout` (5 min); no database transaction spans a provider call — observations, progress and final content are separate short writes
- GEN-22 [o] observation and writing/revision request reasoning effort `low` and voice analysis sends no preference; a per-(model, purpose) operator override may replace the value or omit the wire key (MODEL); completion budgets are per stage through `LLMCompletionBudget` — observation scales with the batch size, writing derives from the requested target length, revision from the larger of that and the content it must re-emit — floored at `LLM_MAX_TOKENS_DEFAULT` (8192) and capped at a multiple of it; reasoning and visible output share the budget
- GEN-23 [o] ordinary start derives the actor from authentication, validates its owned post and one eligible prepared writer plus compatible observer when attached material requires it; published locks and active/not-yet-made/deleted voice guards remain authoritative.
  - one nonterminal ordinary job targets a post; collision returns the actual durable job identity
  - unified tests validate MODEL-30/75 snapshots/entrants and QUOTA-74 plans without targeting or blocking the source post's ordinary job
  - retained legacy experiment records never block new ordinary writing; test publication rechecks owner/lifecycle/version before mutation
- GEN-24 [o] the contact sheet pairs each attached image and video with its persisted observation by exact filename and shows `scene`, `mood`, `visible_text`, `objects` (a video card also `events` and plays the clip inline →VIDEO-9)
  - completed entries appear immediately and the rest say 관찰 대기
  - each non-terminal job snapshot refreshes the post read model
  - thumbnails use only the presigned `view_url` from GetPost, never a `blob:` upload preview
  - on a phone it is one horizontal snap carousel with cards narrower than the strip, a 현재/전체 indicator and no inner vertical scroller, from `sm:` a fixed-width strip
  - the reading view renders the block array directly and never stores or renders canonical HTML
- GEN-25 [o] ordinary Storyline first and Write now actions use the writing brief, await latest required material saves and share published/voice/job guards. Unified tests are a named lower-emphasis route into their own factor/format/material review and start; no saved comparison pair becomes an ordinary-writing prerequisite.
- GEN-26 [o] model A/B preferences are optional common two-entrant test prefills under MODEL-65; no separate editor pair editor or C/D/E comparison form starts work outside the common test flow. An incomplete/duplicate contender selection names its offending field and writes nothing.
- GEN-27 [o] writing/revision freezes the explicit owned voice or absence and its accepted analysis/projection at admission; later source edits or reanalysis cannot change queued prompt material.
  - target-language projection follows LANG-15 and never reads another voice or unaccepted revised learning source
  - compatible source application rechecks assignments/lifecycle under MODEL-36; ordinary generation preserves existing voice-reassignment guards
  - target-length absence remains absence, tag count is concrete, and writing starts no analysis, embedding or judge work
- GEN-28 [o] long model work is a durable `generation_jobs` row: enqueue writes `queued` and returns the id without running the handler in the request; the worker inside the API process moves it `queued → running → done | failed` (I5); terminal rows are never reopened and a retry is a new row
- GEN-29 [o] the queue is the shared credit gate; each enqueuer lists every bounded call with its exact ref/count/budget, including zero reused observation calls and one complete-post write per test entrant. Observer-factor tests count independent observation pipelines as well. Worker context stamps ledger ownership; match decisions have no jobs, calls or admission.
- GEN-30 [o] the row records owner, nullable `post_slug`, frozen voice_id (nullable for source-neutral account-owned test jobs whose contestant profiles are recorded in the frozen test snapshot), kind, stage, exact progress, frozen target language, a structured failure (`reason + params + technical_detail`; deprecated raw text read only for legacy rows), selected observe and write refs as `provider_id/model_id`, a kind-specific JSON payload and created/updated/started/finished timestamps
  - a `generate` payload freezes target language, target length, tag count, template brief, guideline texts, the storyline when written from one, the selected memory texts, the ticked quality-rule texts (`quality_rules`), `write_native_effort`, `observe_files` (carrying presence) and the reusable observation snapshot ← collapsing present-and-empty into absent would decode "reuse everything" as "re-observe everything", a silent double spend
  - a `revise` payload freezes the instruction, content language, template brief, guideline texts, `tag_count` and `write_native_effort`
  - a storyline job's payload freezes the target language, template brief, guideline texts, the selected memory texts, `observe_files` and the reusable observation snapshot, and a storyline request's also the request and the storyline it starts from
  - a writing-test payload names the frozen owner/test revision and its bounded candidate plan; durable progress separates preparing common material from completed entrant outputs
- GEN-31 [o] guards: one non-terminal job may target a post regardless of kind (partial unique index); one non-terminal voice-owned job per `(voice_id, kind)` (a `BEFORE INSERT` trigger, so pre-0009 active rows survive without rewriting statuses), a row also naming a post satisfying both; a row with neither keeps the older `(user_id, kind)` guard; `ErrAlreadyInProgress` carries the active id to attach to; a post-targeted row has a composite FK to `(posts.slug, posts.user_id)` and the guard lookup is owner-scoped; the voice context asks `HasActiveForVoice` before a soft delete and the job context never reads voice tables
- GEN-32 [o] the worker runs with concurrency 1 inside the API process; enqueue sends a best-effort in-process wake and a 1 s fallback poll catches a missed signal; the oldest queued row is picked first with one `UPDATE … RETURNING`; every progress callback is a separate short write through the serialized writer; handler success becomes `done`, a returned error `failed` with a stable owned reason and allowlisted params (a normalized `llm.ProviderError` maps to a model reason while provider prose stays technical detail), a panic `JOB_PANICKED`, and the worker continues
- GEN-33 [o] graceful shutdown stops the in-process worker before HTTP shutdown; a cancelled handler leaves its row running and a handler with an outcome gets one bounded terminal write; boot fails interrupted in-process jobs with JOB_INTERRUPTED before new work, but reconciles a CLIP job durably waiting on a media stage under ARCH-50; experiment boot recovery reconciles its own aggregates, and no recovery automatically repeats an uncertain paid provider call
- GEN-35 [o] post generation has no automatic retry/cancellation, external model queue or partial-text stream; explicit unified-test failed-only retry/cancellation follow MODEL-35/91 and QUOTA, while CLIP keeps its own media policies.
- GEN-36 [o] owner-scoped durable job reads expose a post's actual active job through published job ports; polling stops at terminal state. History also distinguishes the latest relevant ordinary failure and actual content/export readiness without per-row detail fetches. Tests use their own records and never redirect ordinary polling to a pending experiment review.
- GEN-37 [o] `useJob` asks GetGeneration every `POLL_INTERVAL_MS` (2 s) while `queued | running`, stops after `done | failed`, and on either terminal state invalidates the owner query keys the caller supplied, once per job; stage labels and progress templates are catalog keys rendered in the active locale; a failed row exposes its structured failure and an unknown, malformed or legacy value becomes localized `UNKNOWN_FAILURE`, never raw text; `FailureNotice` delegates retry to the owning feature's `onRetry`; progress and error feedback use live-region semantics and the design system's notice roles
- GEN-38 [o] revision is a durable `revise` job: StartRevision validates the authenticated user's post, existing content, a concrete `content_language`, a trimmed non-empty instruction of at most `RevisionInstructionMaxChars` (500) and an enabled explicit write model, freezes the content language (never substituting the newer target), the exact `voice_id` or its absence, the template brief, the guideline texts, the tag count (→GEN-46) and `write_native_effort`, and returns the job id; it uses the same guard, polling, progress, restart and recovery mechanics as generation
- GEN-40 [o] revision uses the frozen accepted voice projection for the content language, the optional template brief, one applicable guideline section, current full PostContent, current semantic-origin context, attached filenames/orientations and the owner's request.
  - make the smallest requested change; preserve unrelated sentences and their origins, title/summary/tags unless requested, exact filenames and the frozen content language; return a complete replacement, never translate implicitly
  - a requested tag change uses the frozen upper bound; unrelated tags remain unchanged
  - guidelines bind only requested/newly touched writing; new meaning follows GEN-80–84 and receives matching origin information
  - absent template/guideline sections contribute no bytes; revision receives no memories (→MEM-22) and cannot infer missing original evidence from prose or style examples

- GEN-41 [o] revision uses the generation parser and validator, filtering IMAGE, GALLERY and VIDEO filenames against a fresh attachment snapshot after the call ← an in-flight deletion must leave no dangling reference.
  - preserve requested attachment reordering; atomically replace canonical content, machine baseline and matching origin review information, preserving content language and moving to review without finalizing
  - invalid canonical output retains prior content and baseline; invalid origin information alone follows GEN-84 and does not discard usable content

- GEN-42 [o] a completed revision records its instruction verbatim as a guideline candidate at this contract's 500-character bound, inside the job's completion path after the revised content is persisted and before the last progress tick, so a failed, cancelled or running revision records nothing
  - it creates no job and calls no provider, and a recording failure never fails the revision ← the result the user is looking at is authoritative
  - deduplication, counts and the pending bound belong to GUIDE, and no candidate ever reaches a prompt
  - `지침으로 저장` after a completed revision saves the user-edited instruction through the standard guideline RPC — an explicit save of user-authored text, nothing learned — approving the matching recorded candidate in the create's own transaction (`AlreadyExists` shown as already-saved)
  - an edited save is a different rule and the original candidate stays pending
  - this adds no revision history
- GEN-43 [o] the revision form is the first row of ②'s dock (→POST-56), shown only when canonical content exists, using the account's explicit write-stage selection
  - disabled for an empty instruction, a missing or pending selection, an unresolved start, another active job, or a deleted or not-yet-made voice (first, with the shared message)
  - every blocker renders above the row
  - the frontend flushes block-content autosave before StartRevision and a save conflict stops the action
  - a failed revision started in this editor session retries with its retained instruction, one resumed from an earlier session shows the failure and enables a new instruction ← the browser cannot reconstruct a private job payload
- GEN-44 [o] naming is a 기본 지침 of the 지침 kind (→GUIDE-41): where the memo names a subject the observation recorded only generically, the writer uses the memo's name — in prose and in IMAGE `alt` and `caption` alike, a caption being written text rather than a transcript of the observation (키티 인형, not 고양이 인형) — while nothing the observations do not show is written as being in the frame ← grounding holds the memo and the observations as one material, so nothing else decides which noun wins a disagreement
- GEN-47 [o] altitude is a 기본 지침 of the 지침 kind (→GUIDE-41): the observations are grounding and placement material rather than a list to describe, no paragraph is required to point at a photo, and visual detail that carries nothing for the post — lighting, walls, ceilings, fixtures, the arrangement of a room — is not written ← covering the observation array entry by entry reads as a caption for every photograph rather than as one post
- GEN-45 [x] the observe prompt receiving the post's title and memo — observation stays context-free
- GEN-46 [o] `tag_count` is a frozen upper bound on generation, requested tag revision and writing-test output; its changed value changes the input hash (→POST-63 →MODEL-30).
  - keep at most the first `tag_count` tags in model order; accept fewer, including none, and never generate padding to meet the bound
  - revision preserves unrequested tags under GEN-40 rather than trimming them for a newer bound
  - legacy queued payloads without the member decode as `POST_TAG_COUNT_DEFAULT` ← a usable paid draft must survive a shorter grounded tag list

- GEN-49 [o] the title 기본 지침 recommends avoiding repeated title forms that vary only one detail and a keyword repeated twice or more within one title.
  - these are product recommendations, not a published Naver numeric penalty threshold or a promise of exposure
  - an authored title form outranks the recommendation, which binds only newly written `<write>` content (→TMPL-52)

- GEN-50 [o] the tag rule is a 기본 지침 of the 지침 kind (→GUIDE-41): select specific tags matching the post and supported by its material, in descending relevance
  - prioritize identifying business, place, brand and product names, then natural combinations such as area + menu, business category or activity, and entity + core topic; use the area's name from a stated station name when it names the same area
  - for a post about eating tteokbokki at 진아분식 near 답십리역, prefer 진아분식, 답십리떡볶이 and 답십리분식; examples are conditional and never supply facts for another post
  - broad labels such as 내돈내산, 일상, 맛집 and 떡볶이 alone rank after available specific tags and never pad the upper bound; duplicate and spacing-only variants do not add a tag
  - invent no area, entity, offering, payment claim or endorsement; when an area is unknown, use supported entity/topic tags without inventing one, and never alter prose to justify a tag
  - a revision applies tag selection only when its request changes tags (→GEN-40); no search-demand data is read anywhere
- GEN-51 [o] a ticked quality rule's text (→QUAL-13) renders at the head of the per-post half, never in the stable prefix, closed by a fixed line saying a conflicting 지침 outranks it ← the ticks differ per post, and the prefix is what the provider's cache and every prompt golden rest on (→MEM-20)
- GEN-52 [o] a frozen template's title area becomes its own instruction inside the template brief, above the body form (→TMPL-50)
- GEN-55 [o] the write answer also carries `nouns`, the distinct nouns the title and the body use, at most 40, in the frozen target language (→QUAL-7 →QUAL-9); a revision keeps the stored nouns ← the write model already reads every word it wrote, so no tokenizer or second call is needed
- GEN-56 [o] Start, StartRevision and applying a write-comparison candidate refuse a published post before any job, snapshot or provider call, beside GEN-23's and GEN-38's preconditions (→POST-74)
- GEN-67 [o] on the direct path (바로 글 쓰기) the write answer opens with `storyline` ahead of the title and the blocks — GEN-68's form, holding every attached photo and video once — and the post then follows it; the post stores it in place of any earlier storyline and ② shows it (→POST-95), and a revision asks for none and changes none ← one call has to set the story before it writes, and the first member of a structured answer is where it does that without a second call
- GEN-68 [o] storyline creation/regeneration is a durable job under GEN-23, the published lock and one-active-job guard.
  - observe as generation does, then use one prepared writer call with memo, observations, answered template, stage-applicable stock rules, all scope-resolved verbatim owner guidelines, opted-in frozen memories and target language; no voice or stock final-post-only title/tag/prose instructions (→GUIDE-15)
  - return ordered target-language plan paragraphs and exact attachment names; each attached photo/video appears once, respecting template places; filter unknown names, and an omitted attachment is taken out (→POST-96)
  - the plan follows GEN-80–84; composition order is not evidence of event order
  - replace storyline and matching origin context, leaving canonical content unchanged ← a plan is not prose in the owner's voice

- GEN-69 [o] the storyline space's AI request is a durable job under GEN-68's preconditions taking a trimmed request of at most `RevisionInstructionMaxChars` (500) and the stored storyline as the owner's own edits left it, with GEN-68's material and every stored observation reused without the picker; it rewrites the storyline alone — neither the post's content nor its observations — and records no guideline candidate (→GUIDE-7)
- GEN-70 [o] writing from a stored storyline freezes that plan and its semantic origins, reuses stored observations without the picker and observes only an included attachment lacking an observation.
  - the plan determines covered subjects and composition order over conflicting guideline order or earlier memo composition; the other material supplies supported detail
  - plan approval or partial editing never promotes AI claims into owner facts, overrules supplied factual evidence or creates unsupported chronology (→GEN-81–83)
  - narrow writer attachments and final filters to the plan's held files, placing each where its paragraph specifies; return no replacement storyline ← the selected plan controls arrangement while its claims keep their evidence

- GEN-71 [o] a storyline changes only through GEN-67's direct write, GEN-68, GEN-69, an applied comparison candidate (→GEN-72) and the owner's own edits in its space (→POST-96): a revision, a hand edit of the post or a finalize never rewrites it, and deleting an attachment takes it out of the paragraph holding it (→POST-18)
- GEN-72 [o] each complete test post carries its candidate's storyline as the direct writer does; stored candidate content/storyline is reused in every bracket round. Explicit compatible model-result application stores both and never writes a storyline merely because a winner was chosen.
- GEN-73 [o] 기억을 통한 감상 추가 is a switchable 기본 지침: when enabled, a frozen `preference` memory may support a visibly identified AI-added impression about a fact already supported by this post.
  - 매웠다 plus a preference for spicy food may suggest 매워서 좋았다; 좋았다 is AI-added meaning, with that memory as its inspectable basis, never an impression directly supplied for this visit
  - supply no objective fact and do not replace an impression the owner already stated; other memory kinds supply facts only (→MEM-5 →GEN-16)
  - include the rule only with opted-in `[기억]`; memory-free and revision prompts carry no such line (→MEM-22)

- GEN-74 [o] a post with 말투 없음 carries no voice bytes: no projection, no ending-mix line, and the template and guideline precedence sentences name no voice ← a sentence pointing at a profile that is not there invites the model to invent one
- GEN-75 [o] 같은 종결어미 세 번 잇지 않기 is a 기본 지침 of the 지침 kind (→GUIDE-41), offered and injected for a Korean target alone inside `[작문 지침]`: no three sentences in a row end with the same ending (`ENDING_MAX_CONSECUTIVE` 2), on every post whether or not it has a voice ← as a fixed line of the voice section it blocked writing that needs its opposite
- GEN-76 [o] each new writing start checks the selected template's required data fields against the saved post answers before a credit hold, snapshot or job row: direct and storyline-following generation, storyline creation and rewrite, and write comparison share the gate (→TMPL-68); a missing field refuses with its first label, while revision of existing content does not gain this gate
- GEN-77 [o] a photo group is one place in the post where 2 … `PHOTO_GROUP_MAX` (3) attached photos of one orientation stand together in order under one caption, laid out as 콜라주 (side by side) or 슬라이드 (one at a time, swiped) ← Naver shows several photos as one collage or slide under one caption, and a post that can only stack single captioned photos reads as a run of photos with nothing between them
  - the cap is three ← a larger collage wraps into rows above one caption, which reads as rows of photos nobody described
  - one orientation: portrait photos group with portrait photos and landscape with landscape, a photo's orientation read from its dimensions on record turned by its rotation (→POST-107), a square photo counting as landscape; the write and revise prompts tell the writer every attached photo's orientation
  - every group carries a non-empty caption
  - the writer decides, with or without a template, which consecutive photos stand as a group, in what order and in which layout; a template photo place's `count` is a suggestion, not a requirement (→TMPL-38)
  - static write/revise rules define group shape and evidence contracts; grouping preference remains a 기본 지침, and group/display order never supplies event chronology (→GUIDE-41 →GEN-82)
  - each attached photo still stands once in the post, alone or inside one group, and a video never joins a group
  - a revision forms, splits or regroups photos only as its request asks, like any other change (→GEN-40)
- GEN-78 [o] a photo group passes the block validation (→GEN-2) and the revision's fresh attachment filter (→GEN-41) photo by photo:
  - a name not attached is dropped, and a photo named twice inside one group keeps its first place
  - the photos left split by orientation — the first photo's orientation first, each keeping its order — and each orientation's photos into the fewest parts of at most `PHOTO_GROUP_MAX`, as even as possible (four photos become two and two) ← dropping a placed photo would lose it from the post silently
  - a part of one photo stands alone as a single photo, and a group left with no photo is dropped
  - every part carries a caption: the first the written caption, each later one the group's alt or, without one, the written caption again; a written group with an empty caption takes its alt ← the alt is the writer's own words about the same photos
  - an absent or unknown layout reads as 콜라주
- GEN-79 [o] a photo's observation also reports `rotation`, the clockwise quarter turn (0, 90, 180 or 270) that makes its scene upright, any other answer reading as 0
  - it is stored, reused and re-observed with the entry (→GEN-8 →GEN-11), and a freshly observed photo takes it as its rotation while the owner has never rotated that photo (→POST-107)
  - a video's entry carries none
  - the observe prompt asks for it as a fact about the frame, beside the observation's other fields (→GEN-7)

- GEN-80 [o] written meaning has three origins, independent of who typed or paraphrased it:
  - owner-input based: meaning explicitly supplied in memo, answers, factual edits or approved memories; an AI paraphrase retaining that meaning keeps this origin
  - photo-derived interpretation: meaning read or inferred from identifiable visual scene/text evidence, distinct from the owner's supplied meaning; attached-video observations use this visual category with the actual video source identified
  - AI-added meaning: an interpretation, explanation, sensory proposal or claim supported by neither supplied owner meaning nor identifiable visual evidence; a plausible model label is not proof of support
  - supplied meaning retains its owner basis; otherwise distinguish visual interpretation from further AI addition, with different meanings inside one sentence separated; a photo cannot establish taste or events outside its observed frame
  - generic instructions, voice examples and template shape are not evidence of this post's actual events; supplied literal text retains its known authoring origin without certifying the experience

- GEN-81 [o] semantic origin follows material through observation, planning, writing, revision and test application.
  - a model-generated plan, reusable AI setting, explicit plan selection or partial manual edit cannot relabel its other claims as owner input
  - distinguish an explicitly supplied new fact from merely approving arrangement or finished wording
  - retained meanings keep their original evidence; unprovable historical origins remain unconfirmed (→POST-114)

- GEN-82 [o] photo upload order, stored-array order, filenames and capture order must never create time passage, venue/seat changes or action sequence.
  - use event chronology only when the owner explicitly supplied it; without it, arrange by subject/content without pretending that arrangement happened in time
  - apply this contract to observation, storyline, writing and revision even with preference guidelines disabled; an AI-added label cannot authorize invented events
  - an actual observed source-time sequence inside video is distinct visual evidence, not photo upload chronology

- GEN-83 [o] AI may propose useful terminology, transitions and richer sensory expression while preserving owner control.
  - style-only rewording keeps the supplied meaning's origin; additional flavor, texture, aroma, interpretation or explanation is visibly AI-added and reviewable
  - 맛있었다 does not itself support sweet flavor or a named aroma; a photo does not establish taste
  - proposals cannot contradict supplied facts/impressions or invent objective experience, price, identity, interaction or chronology prohibited by the active grounding rule and GEN-82
  - owner review does not certify correctness or silently change semantic origin

- GEN-84 [o] canonical content validity and origin-information validity are assessed separately.
  - check allowed categories, existing source references and correspondence to the current text; these checks do not certify semantic support or visual accuracy
  - retain usable canonical content when origin information is absent, malformed or mismatched; mark affected meaning unconfirmed, never fill it with a guessed category
  - do not automatically classify/repair through another provider call, retry, swap models or impose an additional finalize/export gate (→QUOTA-13 →POST-13 →EXPORT-21)
  - existing canonical-output failures and admitted video correction policies remain unchanged

- GEN-85 [o] each prompt stage scopes code-owned instructions and requested model result fields to that stage's declared output, preserving resolved owner guidelines under GUIDE-15.
  - keep meaning, material boundaries, required output fields, precedence, language and failure contracts explicit and consistent across prompt text, schemas, parsers and consumers
  - writing/revision share the flat GEN-1 field/type vocabulary: TEXT/QUOTE prose in `content`, LIST text in `items`, and attachment-specific IMAGE/GALLERY/VIDEO fields; no field example may contradict the accepted schema
  - storyline-only work omits stock final title/tag selection; setting authoring scopes kind/mode rules (→EDIT-24); frozen-story stages omit newly generated plan instructions when the plan cannot be consumed
  - remove duplicate or irrelevant context only when its meaning and applicable consumer behavior are preserved; long useful instructions and distinct examples need not be shortened

- GEN-86 [o] prompt changes are evaluated against preserved writing behavior and owner control, not token reduction alone.
  - use fixed material to check structure, source boundaries, unsupported chronology/facts, mixed-origin phrases, requested-only revision, grounded tags up to the cap, output language and accepted voice
  - record input/output cost, usable-response rate, truncation and origin coverage separately; JSON conformance does not establish correct origins
  - compare instruction-language variants under LANG-29 using human prose/source review; model price grade alone predicts neither compliance nor quality
  - owner-run publication/view observations are a later empirical trial, without automatic publishing, traffic measurement or exposure promises

## flow
- generate: 바로 글 쓰기 | 이 스토리로 글 쓰기 → StartGeneration(preconditions → freeze target, length, tag count, brief, guidelines (기본 지침 first), voice or 말투 없음, the storyline when writing from one, reobserve selection + snapshot → hold credits → job row) → worker(observe frozen set in batches → merged snapshot persists → write (direct: the storyline, then the post along it | along the frozen storyline) → validate → attachment filter (photo groups photo by photo →GEN-78) → content + baseline (+ storyline on the direct path), `review`) → `useJob` polls 2 s → `done`
- storyline: 스토리라인 먼저 | 다시 만들기 | the space's AI request → storyline job(preconditions → freeze target, brief, guidelines, memories, reobserve selection + snapshot, the request and the storyline it starts from → hold credits) → worker(observe what is missing → one writing call → names matched against the attachments) → storyline stored → ② storyline space
- revise: instruction → StartRevision(freeze content language, voice or 말투 없음, brief, guidelines) → worker(revise prompt → validate → fresh attachment filter → content + baseline, `review` → record guideline candidate) → done
- queue: Enqueue(guards → admitter → `queued`) → worker picks oldest(`running`) → progress writes → `done | failed(reason, params, detail)`; boot: interrupted in-process execution → `failed JOB_INTERRUPTED` | durable CLIP media wait → reconcile under ARCH-50

## constraints
- config BE (`internal/platform/config`): `OBSERVE_BATCH_SIZE` 4 (env) · `LLMStageTimeout` 5m · `LLMMaxTokensDefault` 8192 (env) · per-stage completion budgets (typed constants) · `WorkerConcurrency` 1 · `WorkerPollInterval` 1s; `internal/generation`: the stage reasoning policy `DefaultReasoningPolicy` · `BadOutputErrorHeadChars` 200 · `RevisionInstructionMaxChars` 500; `internal/guideline`: `ENDING_MAX_CONSECUTIVE` 2, beside the ending-run 기본 지침's text; `internal/job`: `JOB_INTERRUPTED` `JOB_PANICKED`; FE `shared/config`: `POLL_INTERVAL_MS` 2000 · `REVISION_INSTRUCTION_MAX_CHARS` 500
- prompt text, JSON schemas and every 기본 지침's name and text are code, never config or rows
- schema: `generation_jobs(id, user_id, post_slug NULL, voice_id NULL, kind, status, stage, progress_done, progress_total, target_language, reason, params, technical_detail, observe_model, write_model, payload, created_at, updated_at, started_at, finished_at)` with the post/account partial unique indexes and the voice `BEFORE INSERT` trigger
- placement BE: `backend/internal/generation` (observe, storyline, write, revise, prompts, schemas, handlers; consumer-owned ports to post, voice, template, guideline, llm) · `backend/internal/job` (queue, worker, guards, `PlannedCalls`, `ActiveJobFinder` behaviour)
- placement FE: `entities/generation-job` · `entities/observation` · `features/generate-post` · `features/edit-with-ai` · `features/configure-model-pair` · `widgets/contact-sheet` · `widgets/generation-brief` · `widgets/refine-dock` · `pages/editor`
- tests that pin it: prompt-assembly snapshots (order, a 말투 없음 prompt carrying no voice bytes, each 기본 지침 present only while switched on, the naturalness 기본 지침 for a Korean target only, 기억을 통한 감상 추가 only in a prompt carrying `[기억]`) · block validator and attachment filter with crafted payloads · `ceil(N/4)` observation calls with a stubbed provider · frozen re-observation set and non-shrinking snapshot · no write transaction across a provider call (handler harness) · guard collisions carry the active id · boot sweep reasons · the direct write answer schema orders `storyline` first and the post stores it

## chg
- r26 261007 GEN-14✎ GEN-16✎ GEN-40✎ GEN-41✎ GEN-46✎ GEN-49✎ GEN-50✎ GEN-68✎ GEN-70✎ GEN-73✎ GEN-77✎ GEN-80+ GEN-81+ GEN-82+ GEN-83+ GEN-84+ GEN-85+ GEN-86+ format-only→evidence; exact tags→cap; impressions/memory inference→visible AI; untraced revisions→origins; plan authority→arrangement; title source claim→recommendation; global stock rules→stage rules
