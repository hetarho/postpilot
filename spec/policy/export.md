# Policy — Platform export

Canonical rules that are **currently true** in the code. Source: [plan/08](../plan/08.platform-export.md), built by
job 13, with language provenance from job 32. Every output is derived in the browser from canonical `PostContent`;
no derived format is stored or served by
the API, and the export surface itself never publishes. The separate paired-Mac action is governed by
[publishing.md](publishing.md).

## Shared rules

- Naver, Tistory, standalone site HTML, and Markdown are pure synchronous conversions of the same block array.
  Switching formats makes no request.
- Converters require the post's concrete `content_language`; the panel fails closed when provenance is absent. They
  never substitute a newer target, inspect prose, or translate user/generated content.
- Every image reference comes from `Block.file`. Attachment `view_url` values, API origins, R2 hosts, and image bytes
  are never emitted. Site and Markdown encode the filename as one relative URL path segment so accepted punctuation
  cannot become a query, fragment, or malformed Markdown destination.
- HTML text and attributes are escaped. Markdown image labels escape label delimiters; YAML strings use JSON-compatible
  double-quoted escaping.

## Format contracts

- Naver output is plain text without the title. Blocks are separated by one blank line; image-marker labels follow
  content provenance: `[사진 …]` for Korean and `[Photo …]` for English. The title has its own copy field. The
  markers are unresolvable by SmartEditor ONE on their own, so the panel also renders a **photo strip** beside the
  text — see below.
- Tistory output is an HTML fragment without document wrappers. It begins with the summary, leaves every image `src`
  empty, stores the exact filename in `data-file`, and places a Korean or English replacement instruction comment
  immediately after the image according to content provenance. Comment-closing sequences in unusual accepted
  filenames are neutralized inside the comment context.
- Site output is one standalone document whose `<html lang>` comes from content provenance, with a fixed,
  content-independent inline stylesheet. It includes title, summary, creation date, tags, and article blocks; images
  use URL-safe relative filenames.
- Markdown begins with YAML front matter containing title, creation date (`YYYY-MM-DD`), summary, tags, and canonical
  `language: ko|en`. Body headings, quotes, lists, images, and optional italic captions follow the Plan 08 mapping.

## Clipboard and UI

- The export panel exists for canonical content in both `review` and `finalized` workflows and shows localized
  format-specific guidance. Its preview is read-only and is the exact string passed to the clipboard.
- A successful clipboard write shows “복사됨” for `1500ms`. If the API is absent or rejects, the relevant field is
  focused and fully selected and the manual-copy hint is shown.
- Copy operations are serialized. A tab/content change or newer copy invalidates stale feedback and fallback
  selection, so delayed browser permission results cannot act on a different preview.
- **The Naver format also copies photos, one at a time.** Beneath the text output the panel renders one entry per
  `IMAGE` block, in the same order as the `[사진 …]` markers in that text and derived from the same block array, each
  naming its marker filename so a photo on screen and a marker in the pasted text are matched by eye. The other
  three formats render no strip: they resolve their own images.
- **The clipboard payload for a photo is exactly one `ClipboardItem` carrying `image/png` and nothing else.** It is
  measured, not chosen (owner, 2026-08-31, SmartEditor ONE on macOS Chrome): a `text/html` flavor beside it makes the
  editor prefer the HTML and render a broken image, a second `ClipboardItem` is silently ignored, and a page cannot
  write files to the system clipboard at all. There is therefore **no text fallback** for a photo copy — the text
  output beside it is the fallback the plan already promises.
- The PNG is handed to the `ClipboardItem` as a **promise**, and the write is issued before the bytes arrive:
  awaiting the fetch first spends the user activation WebKit requires, which would make every iOS copy fail.
- The stored photo is a JPEG ([I6] converts every upload in the browser), so it is re-encoded to PNG on demand, for
  the one photo pressed, from the presigned view URL. Nothing encoded here is uploaded, published or persisted.
- A photo failure is stated **on that photo**, and told apart by kind: the browser has no image clipboard, the write
  was refused, the bytes could not be read (an expired view URL — reloading the post remints it), or no photo in the
  post matches that marker. A photo that cannot be read offers no copy control at all rather than one that would
  write an empty image; the same holds for a photo still carrying its local upload preview.

## Configuration

| Value | Owner | Value |
|---|---|---:|
| `COPY_FEEDBACK_MS` | FE `shared/config` | `1500` — shared by the text copies and the per-photo copy |
| site CSS and document shell | FE `features/export-site/config` | fixed code constant |
| format guidance | FE i18n `posts` catalog | Korean/English keys |

## Learning isolation

- Export reads the latest canonical edited `PostContent`; it is available before or after finalization.
- The separate publishing panel is composed after export in 글 완성. It neither changes the export output nor turns a
  copy action into a publication request.
- Rendering, opening a tab, copying through either clipboard path, and feedback timers never freeze a learning
  snapshot, enqueue work, update a profile, or call a provider. `확정`, `확정하고 말투 학습`, and later
  `말투 학습` are separate explicit editor actions.
