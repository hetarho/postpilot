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
  /** The request for the bytes never reached the object: the bucket allows no browser `GET` from
   *  this origin, the request was blocked before it went out, or the network failed. Reloading
   *  the post does NOT help — the fresh URL is blocked the same way — so the caller points at
   *  the text output instead of promising a recovery that cannot happen. */
  | { kind: 'blocked' }
  /** The bytes arrived but never became an image: the presigned URL expired, or the decode or
   *  the encode failed. Reloading the post remints the URL, which is the recovery. */
  | { kind: 'unreadable' }

/** Which of the two read failures happened. They lead to opposite advice, which is the whole
 *  reason they are told apart — and they are told apart by the URL, not by the response. */
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
 *  The stored photo is a JPEG ([I6] converts every upload before it leaves the device), so it is
 *  re-encoded to PNG here, on demand, for the one photo pressed. Nothing encoded here is ever
 *  uploaded.
 *
 *  The fetch needs the bucket to allow a browser `GET` from this origin. `<img>` needs no such
 *  allow, so a photo can render in the panel while being unfetchable here — that asymmetry is
 *  what the `blocked` result exists to name (DEPLOY.md §5). */
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
  let readFailure: ReadFailure | undefined
  const png = loadPng(url).catch((error: unknown) => {
    readFailure =
      error instanceof PhotoUnreachable && !presignExpired(url, Date.now())
        ? 'blocked'
        : 'unreadable'
    throw error
  })
  // The write below is what consumes this promise. This terminal handler exists so a rejection
  // cannot go unhandled on a path where it never gets that far.
  void png.catch(() => undefined)
  try {
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': png })])
    return { kind: 'copied' }
  } catch {
    // Every failure arrives here, including the load's, because the write is what consumes the
    // promise. So a read failure names itself, and a rejected write over readable bytes is the
    // policy refusal that is left.
    return { kind: readFailure ?? 'refused' }
  }
}

/** The request produced no readable response at all. Which of the two read failures that IS cannot
 *  be read off the response — see `presignExpired`. */
class PhotoUnreachable extends Error {}

/** Whether a presigned URL's own lifetime had already run out, answered from the URL and a clock
 *  rather than from the response.
 *
 *  This is not a shortcut, it is the only thing that works: R2 answers an expired read with a 403
 *  that carries **no CORS headers**, so the browser withholds it and `fetch` rejects — exactly as
 *  it does when the bucket allows this origin no `GET` at all (Cloudflare, *R2 → CORS → Use CORS
 *  with a presigned URL*). The two failures are therefore indistinguishable from the response, and
 *  they lead to opposite advice: reload the post, or stop trying and place the photo by hand.
 *
 *  SigV4 states the lifetime in the query (`X-Amz-Date` + `X-Amz-Expires`), so no request is
 *  needed. A URL that carries neither is not presigned and cannot be claimed to have expired. A
 *  wrong client clock costs a less useful message and nothing else — nothing here is a security
 *  decision. */
function presignExpired(url: string, now: number): boolean {
  let query: URLSearchParams
  try {
    query = new URL(url).searchParams
  } catch {
    return false
  }
  const lifetimeSeconds = Number(query.get('X-Amz-Expires'))
  if (!Number.isFinite(lifetimeSeconds) || lifetimeSeconds <= 0) return false
  // `20260904T010203Z` — ISO 8601 *basic* format, which `Date.parse` is not required to accept.
  const signedAt = Date.parse(
    (query.get('X-Amz-Date') ?? '').replace(
      /^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z$/,
      '$1-$2-$3T$4:$5:$6Z',
    ),
  )
  if (Number.isNaN(signedAt)) return false
  return now > signedAt + lifetimeSeconds * 1000
}

async function loadPng(url: string): Promise<Blob> {
  let response: Response
  try {
    response = await fetch(url)
  } catch {
    // `fetch` rejects, rather than resolving with a non-2xx, whenever the response was never made
    // available to this page: no CORS allow from the bucket for this origin, an expired URL whose
    // 403 carries no CORS headers, a request blocked before it went out, or a network fault. The
    // URL is deliberately not repeated in the message — a presigned URL is a capability to a
    // private object, and this string can end up somewhere it is not.
    throw new PhotoUnreachable('photo fetch blocked')
  }
  // A non-2xx that the browser did expose: the request reached the object and the answer was no.
  if (!response.ok) throw new Error(`photo fetch failed: ${response.status}`)
  const bitmap = await createImageBitmap(await response.blob())
  try {
    return await encodePng(bitmap)
  } finally {
    bitmap.close()
  }
}
