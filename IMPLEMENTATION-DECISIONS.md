# Implementation decisions to review (T414–T448, 260928)

Ambiguous points the spec did not settle, resolved in code so the wave could keep moving. Each
line says what was decided and what to change if you want the other way. Tasks skipped because a
choice would be expensive to undo are listed at the end.

## T414 — video template outline grammar, no guide

- **Caption `position`/`align` and badge `align` in the template grammar: ignored, not refused.**
  The task let us pick either. Both parsers (Go `ParseTemplate`, TS `parseClipTemplate`) read them
  as the defaults, the same way a stored `accent`/`pace` outside its list is now read as absent
  (CLIP-113: refuse only what the grammar cannot read). A shared corpus case pins it. The snapshot
  grammar (`Parse`) still refuses an invalid position. *If you want refusal instead:* switch the two
  `template && role === 'caption'` branches to a problem such as `unknown_attribute`.
- **The `ai` entry's text ceiling is kept at 4,000 under a new name.** It used to share
  `GuideChars`; with the guide gone it is `GeneratedChars` / `generatedChars` (same value). No SSOT
  number exists for it. *If you want it tighter* (e.g. the 200-character prompt limit), change the
  one constant in `clip/limits.go` and `clip-design/config/clip-composition.ts`.
- **`composition.guide` i18n key kept.** The task listed it among the guide-editor keys to delete,
  but it is the 형식 안내 text itself (`clipCompositionGuide`); only the editor's keys
  (`guidance`, `node.guide`, `add.guide`, `errors.guide_limit`) went.
- **Snapshots: the guide is stripped from the stored `body` string, not from a `$.Guidance` key.**
  A project's `composition_snapshot_json` holds `{version, body, template_id}`, never a parsed
  document, so migration 0093 removes `<guide>…</guide>` / `<guide/>` from `video_templates` bodies
  and from each snapshot's `$.body`.
- **Cut rhythm now always uses the CDS-37 length targets.** It used to switch them off when a
  template carried a guide. CDS-37's "the storyline, otherwise the owner instruction" is not
  detected yet; that belongs with the clip storyline tasks (T445–T446).
- **Copy tweaks beyond the list:** the `ai` entry help and the stage hint now say "what stands
  here; how belongs to 영상 지침", the unused 글자 정렬 labels and `editor.guidance` went, and the
  video-templates page description no longer says "cut guidance".

## T430 — post template form only, no `<note>`

- **Legend photo/repeat lines switched to the storyline wording here.** T428 kept 흐름/요구하는 on
  its old base; T430's goal (the legend describes photo places along the storyline) was the first
  task to own that wording, so it is 스토리라인/다루는 now.
- **`empty_write` copy now reads "이 자리에 오는 것이 비어 있어요" / "nothing says what goes here"**
  to match the new write label; the task did not list it.
- **The English guide avoids the word "place"** ("photos go here", "the topic here"): the guide
  test forbids it because the retired place slot must never be taught.
- **Migration number 0094**, since T414 took 0093.

## T431 — 기본 지침 (backend)

- **`natural_korean` renders as one bullet with indented continuation lines.** The old baseline was
  a list of `- ` lines; inside `[작문 지침]` each default is one `- ` bullet, so the leading dashes
  were stripped and the lines continue two spaces in. Owner guidelines with line breaks render the
  same way. *If you want each baseline line as its own bullet:* split the text on `\n` in
  `writeGuidelinesSection`.
- **A non-English target reads the Korean texts.** Generation has only `ko`/`en`; the adapter maps
  anything else to Korean.
- **An unknown kind is `GUIDELINE_DEFAULT_NOT_FOUND` too.** The service refuses a kind outside
  `post`/`clip` with the same reason as an unknown key; the RPC reads UNSPECIFIED as POST, so only
  an internal caller can hit it.
- **Test fixtures freeze no defaults unless the test is about them.** The prompt goldens
  (`write_prompt_no_template`, `revise_prompt_no_template`, `write_prompt_memories`) carry no
  `[작문 지침]` section, so they pin the format-only static rules; the 11 defaults are pinned by the
  default and section tests instead.

## T429 — the direct write opens with a storyline

- **A storyline with no paragraph is stored as none.** An answer without `storyline`, a malformed
  one, or one whose paragraphs all drop leaves the post with no storyline (NULL), because an empty
  plan would list every attachment under `taken_out_files`. *If you want "the write had no plan"
  kept distinct from "no storyline yet":* store the empty plan and have the read model skip
  `taken_out_files` when `paragraphs` is empty.
- **Parser details the task left open:** a paragraph with neither text nor a file after bounding
  is dropped (before the 30 cap), and file names match exactly, so `A.JPG` is not `a.jpg`.
- **`StorylineCompletionAllowance` (1,024) lives in `platform/config` beside the write budget**,
  since the write call and the credit hold are both computed there. It is added before the
  native-effort doubling and capped at the ceiling, so a no-target write asks for 9,216 tokens
  and a native-effort one for 18,432. Revisions keep their budget.
- **`MadeWith` lists photos first, then videos**, each in attachment order: the order the write
  prompt names them.
- **A deleted attachment's traces go in one statement, which still follows the row delete.** The
  observation and the storyline name are written together (`UpdatePostAttachmentTraces`), but
  as before this is not the same transaction as the row delete. *If you want it atomic with the
  row:* move both into the store's delete transaction.

## T432 — the 지침 screen lists the 기본 지침

- **The defaults get their own section headed `기본 지침` / `Default guidelines`**, above
  `저장된 지침`, with no help line (per your "no help copy on simple controls" preference). The
  task named no heading.
- **Switched-off rows are dimmed by fading the name and text (`opacity-60`)**; the switch itself
  stays at full strength so its state is readable. A switch is disabled while its save is in flight.
- **The switch hook is `useSetDefaultGuidelineEnabled(ownerId, kind)`**: the task wrote
  `(kind)`, but the cache key is per account. For the clip kind (T439) the cache key currently
  sits under the same `['guidelines', …]` root.

## T433 — storyline jobs (backend)

- **The two start RPCs carry model refs.** `StartStorylineRequest` gained `observe_model` and
  `write_model`, and `StartStorylineRevisionRequest` `write_model`, like `StartGeneration`. The
  task listed only slug and reobserve, but the server keeps no per-post model, so without them the
  write/observe/video preconditions could not be checked.
- **Storyline jobs carry no voice subject.** They are post-targeted (one active job per post), and
  the start still requires an active voice as `Start` does, but the job row does not name the
  voice, since the prompt has none. *If you want voice deletion to cancel them:* pass the voice id
  to `postVoiceWork` in the two enqueue adapters.
- **The storyline request's payload also freezes the stored observations.** The acceptance asked
  for this; the impl notes' field list left it out. What the request shows the model is the
  still-attached files those observations cover; a photo attached later with no observation is
  not shown until the next 다시 만들기.
- **A storyline answer that is missing, malformed or empty fails as `MODEL_OUTPUT_INVALID`.** In
  the direct write an empty storyline is tolerated because the post is the output; in a storyline
  job the storyline *is* the output.
- **The storyline prompt's template section drops the title form and the closing
  template-vs-voice sentence.** It keeps the heading, legend and body form. Its `[작문 지침]`
  closes with the storyline precedence sentence from the impl notes.
- **English wording I wrote for the twins:** task "Plan the storyline of a blog post from the
  material below. Do not write the post yet.", paragraph plan "Write each paragraph as sentences
  that state the plan (it shows …, it tells …).", and the request rule "Revise [현재 스토리라인]
  as [수정 요청] asks. Leave the paragraphs and the photo placement the request does not touch as
  they are, and keep every attached photo and video in exactly one paragraph."
- **Error copy for `POST_STORYLINE_MISSING`:** "아직 스토리라인이 없어요. 먼저 스토리라인을
  만들어 주세요." / "This post has no storyline yet. Make one first."

## T434 — writing from the storyline; the owner's storyline edits

- **The from-storyline write keeps the storyline but updates the nouns.** The acceptance said
  `SetGeneratedContent(..., nil)`, which would also keep the *previous* write's nouns next to new
  content. I pass the new nouns with a nil storyline instead, which leaves the storyline untouched,
  `EditedByHand` included.
- **Storyline-path static rules:** the direct write's rules with the storyline rule swapped for
  your storyline-path rule, the `storyline` member dropped from the answer shape, and "storyline에서"
  in the IMAGE/VIDEO placement lines changed to "[스토리라인]에서". English twin: "[스토리라인] sets
  what this post covers and in what order. Do not write anything the storyline does not cover,
  even when the memo has it, and use the material only to fill in the details of what the
  storyline covers. Place each photo and video once, where the paragraph holding it stands."
- **`[스토리라인]` renders as `1. text (파일: a.jpg, b.jpg)`**; a paragraph with no file omits the
  bracket; line breaks inside a paragraph are folded to spaces.
- **Still requires an observe model whenever the post has attachments**, even if every held
  attachment already has an observation (same rule as an ordinary start).
- **The write budget still includes the 1,024-token storyline allowance** on this path, although no
  storyline is written; the hold is slightly generous rather than a second budget rule.
- **Three new failure reasons:** `POST_STORYLINE_INVALID` (count change, file in two paragraphs,
  text past 1,000, the last with `max`), `POST_STORYLINE_FILE_UNKNOWN` (`file`: not attached, or
  attached after the storyline was made), `GENERATION_STORYLINE_REOBSERVE`. Copy: "스토리라인을
  저장하지 못했어요. 새로고침한 뒤 다시 고쳐 주세요." / "{{file}} 파일은 이 스토리라인에 넣을 수 없어요.
  스토리라인을 다시 만들어 주세요." / "스토리라인으로 쓸 때는 다시 볼 사진을 고를 수 없어요."
- **An identical storyline save is a no-op** (no write, no "edited by hand" mark), so an autosave
  that resends the stored value cannot flip the mark.
- **"Busy" means any active job on the post** (not only content-writing ones), since a storyline job
  would overwrite the edit.

## T435 — ①'s 스토리라인 먼저 · 바로 글 쓰기

- **The ▾ trigger is disabled whenever its only action (A/B 비교) is refused for a reason no brief
  field fixes** — published, a running job, a deleted voice, a pending A/B result — exactly like
  the old A/B button. A setup refusal leaves it live. `ActionMenu` supports disabled rows with a
  visible reason, but this menu does not use it yet.
- **스토리라인 먼저 uses 바로 글 쓰기's checks and, when refused for setup, opens the brief on
  바로 글 쓰기's fields** (write model, observe model), since it runs on the same models.
- **Which step owns a storyline job the editor did not start:** ① while the post has no storyline,
  ② once it has one (다시 만들기). A failed job owned by ② offers no retry until T437.
- **Layout:** phone row 3:7 = 스토리라인 먼저 | [바로 글 쓰기 (fills) + ▾]; from `sm:` up the three sit
  right-aligned at natural width. The ▾ is icon-only, named 다른 방법으로 쓰기.

## T436 — ②'s storyline space

- **Invalid storyline saves are taken back, not retried.** A save refused with
  `POST_STORYLINE_INVALID`, `POST_STORYLINE_FILE_UNKNOWN` or `POST_STORYLINE_MISSING` drops the
  pending edit (and, as with the published lock, the rest of that pending draft), because the server
  will refuse it every time. `POST_BUSY` keeps today's retry.
- **A text edit opens with a pencil and closes with 완료** (Editable + autogrowing Textarea); every
  keystroke is queued. New copy the task did not give: `{{n}}번째 문단 고치기`, `완료`,
  `{{file}} 옮기기` (move control's name), `{{file}} 넣기` (put-back control's name).
- **넣기 opens an action menu of paragraphs** (`ActionMenu`); the move control is the single-choice
  `Menu` as the task said.
- **Tiles:** `Thumbnail`/`VideoTile` gained a `small` (64px) size; a missing view URL shows the
  filename inside the tile.
- **Drag:** tiles are always `draggable`; in practice this is the desktop (`sm:` and up) path,
  since touch browsers don't do HTML5 drag.
- **When a storyline job replaces the storyline, an unsaved local edit of the old one is
  dropped** (the space is read-only while the job runs, so this only affects an edit that was
  already saved).

## T437 — ②'s storyline actions

- **The AI request field stays visible under the heading row even when the space is closed**
  (the dock's heading-row / field / send shape); only the paragraphs fold away.
- **Dialog titles I added:** remake "스토리라인을 다시 만들까요?" with your sentence as the body and
  confirm 다시 만들기; rewrite "이 스토리로 다시 쓸까요?" with your sentence and confirm 다시 쓰기.
  English: "Make the storyline again?" / "Rewrite from this storyline?".
- **The rewrite confirmation is decided from the saved revisions** (`contentRevision !==
  machineBaselineRevision`); block edits not yet autosaved at the moment of the press are flushed
  first but do not by themselves trigger the dialog.
- **A setup refusal (no model chosen) keeps the buttons live and opens the brief**, like ①;
  other refusals disable them with the reason above the row.
- **Send button name:** "스토리라인 수정 요청 보내기" / "Send the storyline request"; the field shows
  a live `n/500` count.

## T438 — 영상 지침 (backend)

- **The guideline service is constructed before the clip generation service** in `cmd/api`, so the
  clip revision's candidate recorder can go into `GenerationDeps`; its template directories are
  still wired after their contexts exist.
- **`GenerationDeps.Candidates` is optional** (nil records nothing), mirroring the post side's
  optional candidate recorder; it would otherwise have forced every clip test harness to pass one.
- **Approving by candidate id checks the kind**: creating a post guideline with a clip candidate's
  id is `GUIDELINE_CANDIDATE_NOT_FOUND`.
- **The migration's Down drops clip guidelines and clip candidates**, since the old shape has no
  place for them.
- **A guideline or candidate stored without a kind reads as `post`** in the store (the service always
  sets one).

## T439 — /video-guidelines, 영상 지침 후보, 영상 지침으로 저장 (frontend)
- **One screen, one widget.** `/guidelines` and `/video-guidelines` both render `widgets/guideline-directory` with a `kind` prop (FSD forbids a page importing a page). The page copy moved from `pages/guidelines/config` into the widget.
- **Clip copy I wrote:** the page description ("영상 지침은 영상의 구성과 자막에서 피해야 할 내용과 주의할 점을 정해요…"), the empty state example `자막에 가격을 적지 않기`, `저장된 영상 지침`, candidate `요청한 영상 보기` / `요청한 영상이 삭제됐어요`, and the save dialog description. The create sheet's field label and submit stay `지침` / `지침 만들기`; only the trigger and the title say `새 영상 지침`.
- **When 영상 지침으로 저장 appears:** only after the `revise_clip` job that this mounted dock started (matched by job id) reaches `done`. After a reload, or for a job started in another tab, the button is not offered. The request remains in the 영상 지침 후보 queue instead.
- **Save-as scope:** 전역 by default. 이 영상의 템플릿 「name」에만 appears only while the project's video template still exists in the directory.
- **Nav icon:** lucide `ListVideo`.

## T440 — clip writing calls carry 영상 지침; no content checks
- **How the quote binds the 영상 지침:** their digest rides `GenerationPricing.GuidelinesDigest`, the same way the recovery digest already does. `QuoteInputDigest` and `RevisionInputDigest` hash the pricing, so they bind it. The store recomputes the digest inside the link transaction from the saved pricing, so it never has to read the guideline context. The alternative was a new digest parameter that the store would have to resolve itself.
- **Where the block sits:** `[영상 지침]` is the last thing in each system prompt: after the fixed rules and after a revision's own block. This keeps the provider-cached prefix identical for every account. A clip with no guidelines adds no bytes.
- **Recovery:** a recovered plan written under different 영상 지침 is not reused (they join `planRecoveryDigest` like the instruction). The observations are still reused.
- **Payload versions:** generation payload 6 and revision payload 2. Jobs frozen under the older versions still run, with no guidelines.
- **Citation keys:** a caption that still sends `observation_refs` or `fact_refs` is refused by the closed contract, like any unknown key. The flow's cuts keep their citations.
- **Retired notices:** the five content-check reasons were deleted from the frontend map. A plan stored before this change shows them as the generic "unknown" detail.
- **Also changed:** the narration line "A caption may name any item it has a fact for" now reads "…any item". The "grounded shorter row" wording became "shorter row".

## T445 — 바로 만들기 opens with a storyline; the narration follows it (backend)
- **A total size cap on the storyline (not in the spec):** besides 30 paragraphs and 1000 characters each, the kept storyline text is capped at **12,000 bytes** (about 4,000 Hangul characters). The input allowance is measured in bytes (64,000 minus a margin). The widest narration request is already about 29KB, and a 30×1000-character Korean storyline alone is about 90KB. So measuring that worst case (CLIP-90) would refuse every generation. Paragraphs past the cap are dropped, the same way the post side drops paragraphs past its bounds. Tell me if you want a different number, or a pricing change that raises the allowance instead.
- **How the storyline travels:** it rides the plan in memory only and is never encoded into the stored plan JSON. It is kept in the recovery state (so a continuation keeps it) and in the attempt result, then written to `clip_projects.storyline_json` in the same transaction as the plan. A plan-only generation is not staged, so `clip_attempt_results` needed no column.
- **A generation that writes no storyline keeps the stored one.** An empty value leaves the column alone, like the post side's "nil keeps". This is what T446's "build from the storyline" needs.
- **Prompt wording beyond the notes:** the flow prompt's "ordered cuts, and nothing else" became "its storyline, then ordered cuts, and nothing else". "this response carries no text" became "beyond the storyline, …". The storyline sentence adds "Write the storyline in the language of the observations; at most 30 paragraphs of at most 1000 characters each": the flow prompt never names the project language, but the observations are written in it. The narration's "along storyline" sentence and its `storyline` payload appear only when there is a storyline, so a request without one stays as it was.
- **Revision flow contract:** it is derived from the flow schema by deleting `storyline`. Its prompt contract text now lists keys alphabetically; the content is the same.
- **`added_source_ids`:** GetClipProject compares against the project's current source batch. Other responses (list, create, update) compare against the analysed sources, to avoid an extra read.
- **A storyline that keeps nothing after bounding is none;** the flow is still admitted.

## T446 — clip storyline call, storyline request, building from the storyline, owner edits (backend)
- **The storyline job saves the analysis with the storyline.** "Saves the storyline alone" is read as "no plan and no result": the project also keeps the analysis it read, because the storyline names scenes by observation id, and both the owner-edit check and "taken out" need those scenes. The saved plan and the result are untouched.
- **Pricing shape:** a `Storyline` flag on the clip pricing means one writing call, priced on the flow's policy, with no narration after it. The quote response is the same `QuoteClipGenerationResponse`, with the call labelled `storyline`. 다시 만들기 is the same `StartClipStoryline` call. It overwrites an owner-edited storyline, which then reads as not edited by hand.
- **The storyline request** uses the revision quote store and links through the revision path, like the plan revision. It records kind `storyline` with the owner's words. 스토리라인 먼저 and 이 스토리로 만들기 record the instruction, as a generation does. The storyline request is not recorded as a guideline candidate.
- **Owner-edit rules:** same paragraph count; scenes only from the made-with sources' current observations; no scene twice; no empty paragraph; at most 1000 characters per text; at most 12,000 bytes of text in total (the T445 cap). Anything else is refused as `CLIP_STORYLINE_INVALID`.
- **Building from the storyline:** the flow call sees only the scenes the storyline holds, under their original ids, plus the paragraphs. The prompt says the storyline's order and pace outrank the instruction's. A cut on any other scene is removed with a new notice code, `storyline_scene`; the frontend shows the generic detail for it until T447 gives it copy.
- **`plan_edited_by_hand`** follows the spec formula (`edit_plan_revision != generated_plan_revision`). Caption pace, accent and preset changes already bump the plan revision, so they also read as "edited by hand" and trigger CLIP-180's confirmation. Tell me if only cut/caption edits should count.
- **Infrastructure the new job kinds needed:** migration 0101 rebuilds `generation_jobs` (0063's method) so the cancellation CHECKs and the quote trigger admit `storyline_clip` and `revise_storyline_clip`. The job queue gives both kinds the `prepare` first stage. The new reasons are `CLIP_STORYLINE_MISSING` (256) and `CLIP_STORYLINE_INVALID` (257), with ko/en copy.
- **Removing a source** happens when a new batch leaves it out: in the same transaction, its scenes leave every paragraph and it leaves `MadeWithSources`. Paragraph texts stay.

## T447 — clip ①'s 스토리라인 먼저 · 바로 만들기, ②'s storyline space (frontend)
- **When the quote is read:** only when its button's popover opens (a sheet on a phone). The old inline approval quoted as soon as ① was ready. ①'s buttons are always `스토리라인 먼저` and `바로 만들기`; the `생성`/`다시 생성` labels are gone from ①.
- **Approve labels:** `최대 N 크레딧 · 승인하고 스토리라인 만들기` for the storyline call; 바로 만들기 keeps `…승인하고 생성`. The priced line reads `스토리라인 작성 1회`.
- **Scene frames:** T444's still-frame helper doesn't exist yet, so a scene tile is a muted `<video>` seeked to the scene's start, the way `ClipCutSourceFrame` works. With no footage it shows the scene's number and observed event. "장면 N" counts across all sources in analysis order.
- **Autosave:** 600 ms after the last edit, and on leaving. A new storyline from the server replaces the draft whenever no owner edit is waiting to be saved. The space is read-only while any job runs, not only storyline jobs.
- **Layout:** the space sits above the correction workspace when there is a plan, and replaces the "no plan yet" message when there isn't one. The ②/③ waiting messages now name ①'s two actions.

## T448 — clip ②'s storyline actions (frontend)
- **Order:** approve first, then confirm. The confirmation dialog appears after the approval, when the approved work would replace hand edits; cancelling leaves the quote unused. A build with no plan never asks.
- **Only the two confirmations the spec names:** 다시 만들기 over a hand-edited storyline, and 이 스토리로 다시 만들기 over a hand-edited plan. The AI storyline request asks nothing, even over a hand-edited storyline.
- **The storyline request** runs with ② on screen, like a plan revision, and shows its progress where the field was. Its approve label is `최대 N 크레딧 · 승인하고 스토리라인 고치기`, and the request record labels it `스토리라인 수정 요청`.
- **Batch:** 다시 만들기 and the build are quoted against the project's ready batch (this session's upload or the retained originals). They are disabled while there is none.

## T443 — video-template builder rows open in place
- **The open row follows its entry when moved.** It stays open on the entry it belongs to, including a row inside a moved group. Before, a reorder closed everything.
- **A new entry opens and focuses its first field**, which for a text entry is its first control. Pressing its row button after adding closes it, like any open row.

## T441 — a video template's starting design selection
- **Migration 0102 stores `''` / `''` / `[]` for existing templates**, as 0062 does for projects. The wire answers a template's unnamed preset as the shared default (intro A, outro B), which is also what a project made with it takes. The editor never sees "none", and old rows need no backfill.
- **When a request names both a template change and presets, the template wins.** ① sends its whole draft on every autosave, so the old presets always travel with a template change. The server copies the template's three over them in the same write, and the ① draft adopts them on the spot, so the next autosave does not push the old selection back. At creation, an explicit preset still beats the template's; /clips/new sends none, so it mints in the template's selection.
- **Caption pace and accent stay the project's own.** A template gives only the two presets and the styles, per the task. The legacy `intro`/`pace`/`accent` root attributes still in some template bodies are not read for this.
- **The editor's caption-style checkboxes show names only.** ①'s style samples are drawn for a project (`GetClipCaptionStyleSamples` takes a project id), and a template has none. The list sits inside the preview section, under its intro/outro selectors. As with ①'s list, ticking keeps the catalogue order. Following the no-help-copy rule, there is no "none selected means bold" line. *If you want samples:* the sample RPC needs a project-less variant.
- **The docked 저장 always sends all three**, even when unchanged, the same way it always sends the name and the body.

## T442 — the video-template preview plays an illustrative timed clip
- **How many captions fit:** the span between the regions is split evenly into `ceil(span / 4 s)` captions. Every caption is then at most 4 s. The shortest span the bounds allow (15 s minus 2.5 s minus 3 s = 9.5 s) still gives each at least 3 s, so no 3 s floor is needed in code.
- **The preview draws a style's type, stroke and shadow only.** The frame is the preview's own SVG, not the renderer, so a style's colour, plate and motion are not drawn. The drawing is enough to tell the styles apart as the captions rotate. *If you want each style exact:* the preview would need the renderer's caption samples, and those are per project today (same gap as T441's checkboxes).
- **Info entries keep their own timing, clipped to the span between the regions.** CLIP-170 allows no other text beside the intro or the outro, and names only captions for the span between. Info entries are neither, so they stay where they were authored, cut out of the region spans.
- **A caption entry repeated per cut** counts once per cut, in outline position and then cut order. Entries that do not fit are left out.
- **The old single sample line (`자막은 이렇게 보여요`) is gone.** Its place is taken by the numbered `샘플 자막 N` sentences.

## T415 — clip ① and ② behave and speak as the spec decides
- **One count, the server's:** the instruction is now counted in Unicode characters, spaces and punctuation included, on both sides. The two request fields, the plan revision request and the storyline request, are counted the same way because the server bounds them identically. The answers keep CDS-20's count, since the server counts those that way too.
- **Nothing in ① waits for the autosave.** ①'s two actions and the source picker are no longer gated on `dirty`, `synced` or an autosave in flight; an in-flight save used to close an open quote too. Each committing action flushes ①'s queue first, and the storyline request now does as well. *Not done, and outside F62:* a storyline paragraph edit still waiting for its 600 ms autosave is not flushed before 이 스토리로 만들기 or a storyline request. It is logged in STATE.
- **When the reselection line shows:** only when a retained original reports `expired`, `missing` or `cleanup_pending` and no ready batch exists, and only after a result or a failed or cancelled job, as before. The copy no longer claims originals are never kept.
- **"What restarts" line:** when the latest job failed or was cancelled and the quote reports no recovery, the approval says `이전 시도에서 이어 쓸 수 있는 작업이 없어 분석부터 다시 해요.` Both it and the reuse line now stand outside 요금 자세히.
- **No leave dialog in ②, and no lost edits:** the page's route blocker is gone, along with the tab-close prompt. In its place, an edit still waiting for its autosave is sent when the correction unmounts or the page is hidden. On a tab close that send is best effort. The reload-discard dialog inside the correction workspace stays; it is not a leave confirmation.
- **Copy:** 항목 is the neutral word in `source.boundItem`, since the picker is not given the group's label. `source.saveFirst` now reads "클립 설정을 마치면 원본 영상을 선택할 수 있어요", which is true whether the title is missing or the template needs applying.

## T444 — ②'s flow simulation over still cut frames
- **The control is an icon button** on the frame's top-right row, before the info control, like the audio and refresh controls. Its accessible name is `흐름 보기` / `영상 보기`. The flow view hides the play overlay and the audio control, since it has no playback or sound. Refresh stays.
- **A still is a paused `<video>` at the cut's source start**, the same approach as `ClipCutSourceFrame`. A canvas capture could taint on a cross-origin retained original. Only the cut under the playhead is mounted, with no prefetch of the next cut's frame. While its footage loads, or when none can be had, the cut shows `컷 N` on the canvas ground. *If scrubbing across many cuts feels slow:* mount the next cut's still in advance, as the video preview mounts its incoming cut.
- **In a transition overlap the still is the incoming cut**, since a still has no fade.
- **Overlays are the server's own assets**, the ones the video preview already fetches. Each is drawn whole and at rest while its interval holds the playhead. A sequence style is its one representative frame, which is how the server hands it over.
- **The info control's new line** (`멈춘 장면으로 흐름만 보여줘요. 실제 렌더와 다를 수 있어요.`) stands in both views, next to the existing parity line.
