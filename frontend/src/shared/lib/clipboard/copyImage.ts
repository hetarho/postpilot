import { encodePng } from '../image'

/** Why a photo did not reach the clipboard, so the caller can say WHICH thing went wrong on the
 *  photo the user pressed rather than showing one message for four different situations. */
export type CopyImageResult =
  | { kind: 'copied' }
  /** `navigator.clipboard.write` or `ClipboardItem` is absent — the browser has no image
   *  clipboard at all. Nothing to retry. */
  | { kind: 'unsupported' }
  /** The API exists and the write was rejected: a non-secure context, a permission policy, or a
   *  gesture the browser did not consider user-initiated. Retrying can work. */
  | { kind: 'refused' }
  /** The bytes never became an image: the presigned URL expired, the fetch failed, or the decode
   *  or the encode did. Reloading the post remints the URL, which is the recovery. */
  | { kind: 'unreadable' }

/** Puts ONE photo on the system clipboard as a single `image/png` flavor.
 *
 *  The payload shape is measured, not chosen (owner, 2026-08-31, SmartEditor ONE on macOS
 *  Chrome), and it is the whole point of this function:
 *  - exactly ONE `ClipboardItem`, because a second one is silently ignored;
 *  - carrying `image/png` and NOTHING else. Adding a `text/html` flavor makes the editor prefer
 *    the HTML and render a broken image, which is worse than not copying at all. There is
 *    therefore no text fallback here — `copyText`'s select-the-field path has no analogue for an
 *    image, and the export panel's text output is the fallback the plan already promises.
 *
 *  The stored photo is a JPEG ([I6] converts every upload before it leaves the device), so it is
 *  re-encoded to PNG here, on demand, for the one photo pressed. Nothing encoded here is ever
 *  uploaded.
 *
 *  The fetch is readable and the canvas untainted because the bucket already allows `GET` from the
 *  frontend origin (DEPLOY.md §5). A tainted canvas in some environment is a bucket-configuration
 *  defect, not something to work around by proxying bytes through the API. */
export async function copyImage(url: string): Promise<CopyImageResult> {
  if (
    typeof navigator === 'undefined' ||
    !navigator.clipboard?.write ||
    typeof ClipboardItem === 'undefined'
  ) {
    return { kind: 'unsupported' }
  }
  // The PNG is handed to `ClipboardItem` as a PROMISE, and `write` is called before it settles.
  // That is not a style choice: awaiting a fetch first spends the user activation the write needs,
  // and WebKit then rejects every copy — on iOS the control would fail permanently while telling
  // the user to try again. The payload is unchanged; only when the bytes arrive is.
  let readFailure = false
  const png = loadPng(url).catch((error: unknown) => {
    readFailure = true
    throw error
  })
  // The write below is what consumes this promise. This terminal handler exists so a rejection
  // cannot go unhandled on a path where it never gets that far.
  void png.catch(() => undefined)
  try {
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': png })])
    return { kind: 'copied' }
  } catch {
    // The two failures are told apart by which one happened, not by the error: a rejected write
    // over readable bytes is a policy refusal the user can retry, while unreadable bytes are an
    // expired URL that reloading the post fixes.
    return { kind: readFailure ? 'unreadable' : 'refused' }
  }
}

async function loadPng(url: string): Promise<Blob> {
  const response = await fetch(url)
  if (!response.ok) throw new Error(`photo fetch failed: ${response.status}`)
  const bitmap = await createImageBitmap(await response.blob())
  try {
    return await encodePng(bitmap)
  } finally {
    bitmap.close()
  }
}
