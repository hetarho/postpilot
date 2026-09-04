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
  /** The page may not READ the pixels it is displaying: the photo was painted from a response
   *  the bucket did not mark readable by this origin, so it is origin-unclean and the browser
   *  refuses the bitmap. Reloading the post does NOT help — the fresh URL is refused the same
   *  way — so the caller points at the text output instead of promising a recovery that cannot
   *  happen. */
  | { kind: 'blocked' }
  /** The element carried no usable pixels, or the encode failed. The photo on screen is what is
   *  copied, so this is a photo that never finished painting — reloading the post remints its
   *  URL, which is the recovery. */
  | { kind: 'unreadable' }

/** Which of the two read failures happened. They lead to opposite advice, which is the whole
 *  reason they are told apart. */
type ReadFailure = 'blocked' | 'unreadable'

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
 *  It takes the RENDERED ELEMENT, not the URL. Re-downloading the photo was the entire failure
 *  the user hit: a view URL is presigned for minutes, the panel outlives that, and a dead URL
 *  fails the copy while the photo it names is still on screen — right-clicking the same photo
 *  copied it fine, because the browser was reading the pixels it already had. This reads those
 *  same pixels: after the photo has painted, the copy no longer depends on the URL, on the
 *  network, or on the clock.
 *
 *  The price is that the element must be CORS-loaded (`crossOrigin`), or the canvas the encode
 *  draws on is tainted and the browser refuses to hand the bytes back — that refusal is what
 *  `blocked` names (DEPLOY.md §5). The stored photo is a JPEG ([I6] converts every upload before
 *  it leaves the device), so it is re-encoded to PNG here, on demand, for the one photo pressed.
 *  Nothing encoded here is ever uploaded. */
export async function copyImage(image: HTMLImageElement): Promise<CopyImageResult> {
  if (
    typeof navigator === 'undefined' ||
    !navigator.clipboard?.write ||
    typeof ClipboardItem === 'undefined'
  ) {
    return { kind: 'unsupported' }
  }
  // The PNG is handed to `ClipboardItem` as a PROMISE, and `write` is called before it settles.
  // That is not a style choice: awaiting the encode first spends the user activation the write
  // needs, and WebKit then rejects every copy — on iOS the control would fail permanently while
  // telling the user to try again. The payload is unchanged; only when the bytes arrive is.
  let readFailure: ReadFailure | undefined
  const png = readPixels(image).catch((error: unknown) => {
    readFailure = originUnclean(error) ? 'blocked' : 'unreadable'
    throw error
  })
  // The write below is what consumes this promise. This terminal handler exists so a rejection
  // cannot go unhandled on a path where it never gets that far.
  void png.catch(() => undefined)
  try {
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': png })])
    return { kind: 'copied' }
  } catch {
    // Every failure arrives here, including the read's, because the write is what consumes the
    // promise. So a read failure names itself, and a rejected write over readable bytes is the
    // policy refusal that is left.
    return { kind: readFailure ?? 'refused' }
  }
}

/** The pixels already on screen, re-encoded as PNG. No request is made.
 *
 *  `createImageBitmap` is what enforces the origin-clean rule, and it also rejects for an element
 *  that has not finished loading — the two cases this function can fail with, told apart by their
 *  error below rather than by pre-checking `complete`/`naturalWidth`, which describe the element
 *  and not the decision the browser is about to make. */
async function readPixels(image: HTMLImageElement): Promise<Blob> {
  const bitmap = await createImageBitmap(image)
  try {
    return await encodePng(bitmap)
  } finally {
    bitmap.close()
  }
}

/** Whether the browser refused to hand back pixels it considers not readable by this origin.
 *
 *  Both ends of the read raise it — `createImageBitmap` on an origin-unclean element, and the
 *  canvas encode on a tainted canvas — and both are the same fact about the bucket, so the name
 *  is matched rather than the throw site. It is matched on `name` and not with `instanceof`,
 *  because the encode path throws whatever its `OffscreenCanvas` or `HTMLCanvasElement`
 *  implementation throws. */
function originUnclean(error: unknown): boolean {
  return (error as { name?: unknown } | null)?.name === 'SecurityError'
}
