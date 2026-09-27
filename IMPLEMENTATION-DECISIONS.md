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
