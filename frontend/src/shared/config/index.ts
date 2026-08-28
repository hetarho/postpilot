/** Build-time environment. VITE_* values are baked into the bundle, so only ever
 *  public configuration belongs here — never a secret. */

/** Backend origin for built assets. In dev this is unused: Vite proxies '/api' to the
 *  backend (vite.config.ts), which also sidesteps CORS. */
export const API_URL = import.meta.env.VITE_API_URL ?? ''

/** How long a resolved session is trusted before the route guard re-checks it with the
 *  server. Zero would cost a round trip on every navigation; Infinity would let a
 *  session revoked elsewhere (another tab, an expiry, an operator) keep rendering
 *  signed-in screens until the user reloads. */
export const SESSION_STALE_MS = 30_000

/** How long the editor waits after the last keystroke before saving the draft (PRD F-2:
 *  the memo has to survive a phone that kills the tab). Short enough that a force-quit
 *  costs at most a second of typing, long enough that a fast typist is not one request
 *  per character. */
export const AUTOSAVE_DEBOUNCE_MS = 1_000

/** First delay before retrying a failed autosave; doubled per attempt up to the cap.
 *  The cap is what keeps a tab left open on a dead network from turning into a request
 *  loop while still recovering on its own once the network returns. */
export const AUTOSAVE_RETRY_BASE_MS = 1_000
export const AUTOSAVE_RETRY_MAX_MS = 30_000

/** How long the editor's cross-route handoff may sit unread
 *  (`pages/editor/model/editor-handoff.ts`). It is meant to be picked up a tick later by
 *  the editor the mint navigation mounts; anything older belongs to a navigation that
 *  never happened, and applying it would put stale text back into a post that has since
 *  moved on. */
export const EDITOR_HANDOFF_TTL_MS = 5_000

/** The photo pipeline (PRD F-2, §6.2). Every photo is decoded, downscaled and re-encoded
 *  in the browser before it is uploaded ([I6]); these are the knobs of that step. */

/** Pre-conversion size cap, checked at selection. Advice to the user, not a security
 *  boundary — the server enforces its own cap on what actually lands. */
export const UPLOAD_MAX_FILE_MB = 25

/** Compared case-insensitively against the extension of the selected file. Anything else
 *  is listed as skipped, never uploaded. */
export const UPLOAD_ALLOWED_EXTENSIONS = ['jpg', 'jpeg', 'png', 'webp', 'heic', 'heif'] as const

/** The long edge a photo is downscaled to. Smaller images are never upscaled. */
export const IMAGE_MAX_LONG_EDGE_PX = 1024

/** JPEG encoder quality (0–1) for the uploaded copy. */
export const IMAGE_JPEG_QUALITY = 0.85

/** How many photos may be decoding/resizing at once. Each decoded phone photo is tens of
 *  megabytes of pixels until it is downscaled, so eight at once would push a phone into
 *  swapping; one at a time leaves the browser's own parallel JPEG decoder idle. */
export const UPLOAD_CONVERT_CONCURRENCY = 2

/** How long the model catalog is trusted before it is re-asked. The registry only
 *  changes when the API restarts with an edited providers.yaml, so refetching it on
 *  every mount buys nothing; a few minutes means a new model shows up without a reload
 *  while the dropdowns stay instant. */
export const MODEL_CATALOG_STALE_MS = 5 * 60_000

/** Generation jobs are durable on the server; polling only observes their state. */
export const POLL_INTERVAL_MS = 2_000

/** Maximum natural-language revision instruction length. Mirrored from the backend
 * generation context so the field stops before the authoritative RPC validation. */
export const REVISION_INSTRUCTION_MAX_CHARS = 500

/** Immediate client feedback for voice samples. The backend remains authoritative. */
export const VOICE_SAMPLE_MIN_CHARS = 200

/** How long the HEIC decoder worker stays alive after its last file. Its WASM heap does
 *  not shrink after a 12 MP decode, so it is not kept for a whole session; the chunk is
 *  in the browser cache, so bringing it back for the next batch is cheap. */
export const HEIF_DECODER_IDLE_MS = 30_000
