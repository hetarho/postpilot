# Policy — Platform export

Canonical rules that are **currently true** in the code. Source: [plan/08](../plan/08.platform-export.md), built by
job 13. Every output is derived in the browser from canonical `PostContent`; no derived format is stored or served by
the API and postpilot never publishes to a platform.

## Shared rules

- Naver, Tistory, standalone site HTML, and Markdown are pure synchronous conversions of the same block array.
  Switching formats makes no request.
- Every image reference comes from `Block.file`. Attachment `view_url` values, API origins, R2 hosts, and image bytes
  are never emitted. Site and Markdown encode the filename as one relative URL path segment so accepted punctuation
  cannot become a query, fragment, or malformed Markdown destination.
- HTML text and attributes are escaped. Markdown image labels escape label delimiters; YAML strings use JSON-compatible
  double-quoted escaping.

## Format contracts

- Naver output is plain text without the title. Blocks are separated by one blank line; image markers are exactly
  `[사진 <file> — <caption>]` or `[사진 <file>]`. The title has its own copy field.
- Tistory output is an HTML fragment without document wrappers. It begins with the summary, leaves every image `src`
  empty, stores the exact filename in `data-file`, and places `<!-- <file> 업로드 후 src 교체 -->` immediately after
  the image. Comment-closing sequences in unusual accepted filenames are neutralized inside the comment context.
- Site output is one standalone Korean HTML document with a fixed, content-independent inline stylesheet. It includes
  title, summary, creation date, tags, and article blocks; images use URL-safe relative filenames.
- Markdown begins with YAML front matter containing title, creation date (`YYYY-MM-DD`), summary, and tags. Body
  headings, quotes, lists, images, and optional italic captions follow the Plan 08 mapping.

## Clipboard and UI

- The export panel exists only for a post with canonical content and shows the format-specific guidance defined in
  code. Its preview is read-only and is the exact string passed to the clipboard.
- A successful clipboard write shows “복사됨” for `1500ms`. If the API is absent or rejects, the relevant field is
  focused and fully selected and the manual-copy hint is shown.
- Copy operations are serialized. A tab/content change or newer copy invalidates stale feedback and fallback
  selection, so delayed browser permission results cannot act on a different preview.

## Configuration

| Value | Owner | Value |
|---|---|---:|
| `COPY_FEEDBACK_MS` | FE `shared/config` | `1500` |
| site CSS and document shell | FE `features/export-site/config` | fixed code constant |
| format guidance | FE `widgets/export-panel/config` | fixed Korean copy |

## Learning isolation

- Export reads the latest canonical edited `PostContent`; it does not require finalization.
- Rendering, opening a tab, copying through either clipboard path, and feedback timers never freeze a learning
  snapshot, enqueue work, update a profile, or call a provider. **Finalize and learn** is a separate editor action.
