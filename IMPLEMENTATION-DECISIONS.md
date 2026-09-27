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
