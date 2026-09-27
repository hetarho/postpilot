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
