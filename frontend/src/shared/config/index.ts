/** Build-time environment. VITE_* values are baked into the bundle, so only ever
 *  public configuration belongs here — never a secret. */

/** Backend origin for built assets. In dev this is unused: Vite proxies '/api' to the
 *  backend (vite.config.ts), which also sidesteps CORS. */
export const API_URL = import.meta.env.VITE_API_URL ?? ''

/** Theme is a browser-local interface preference, never deployment or account state. */
export const THEME_PREFERENCE_STORAGE_KEY = 'postpilot.theme' as const
export const DEFAULT_THEME_PREFERENCE = 'system' as const

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

/** A leaderboard entry remains provisional until this many pairwise verdicts. */
export const LEADERBOARD_MIN_MATCHES = 3

/** Generation jobs are durable on the server; polling only observes their state. */
export const POLL_INTERVAL_MS = 2_000

/** Publishing has its own durable queue and a shorter live-status projection. These
 * values are public display/polling hints only; server leases remain authoritative. */
export const PUBLISH_JOB_POLL_MS = 2_000
export const PUBLISH_AGENT_STALE_MS = 30_000

/** Maximum natural-language revision instruction length. Mirrored from the backend
 * generation context so the field stops before the authoritative RPC validation. */
export const REVISION_INSTRUCTION_MAX_CHARS = 500

/** The clear space a popover panel keeps from either viewport edge when its anchor would push it
 *  past one. It matches the page's own `px-4` gutter, so a corrected panel lines up with the
 *  content column instead of floating against the glass. */
export const POPOVER_VIEWPORT_GUTTER_PX = 16

/** The `mt-2` / `mb-2` a popover panel keeps between itself and its trigger, as a number, so the
 *  height measurement can subtract the gap the CSS is about to add. */
export const POPOVER_TRIGGER_GAP_PX = 8

/** The shortest a popover panel is squeezed to before it stops honouring the room it measured.
 *  Below roughly three rows a scroller is worse than a panel that overhangs the viewport edge a
 *  little, and a trigger that close to the edge is a layout bug to fix at the call site. */
export const POPOVER_MIN_PANEL_PX = 160

/** How long successful clipboard feedback remains visible. */
export const COPY_FEEDBACK_MS = 1_500

/** How long a press must be held before it counts as a long press. Short enough that a
 *  deliberate hold does not feel unresponsive, long enough that a slow tap or a scroll
 *  that starts on the control is not mistaken for one. */
export const LONG_PRESS_MS = 600

/** The app-wide TanStack Query defaults, applied in app/providers/query-client. They are a
 *  conservative floor: any query with its own freshness or failure policy overrides them at
 *  its call site. The stale window is deliberately twice SESSION_STALE_MS — the session is
 *  the one query whose staleness gates navigation, so it re-checks while ordinary data is
 *  still trusted. One retry covers a dropped connection without turning a genuine server
 *  refusal into a multi-second wait. */
export const QUERY_STALE_MS = 60_000
export const QUERY_RETRY_COUNT = 1

/** Immediate client feedback for voice samples. The backend remains authoritative. */
export const VOICE_SAMPLE_MIN_CHARS = 200

/** Progressive voice-learning display/input mirrors. No interval is present because
 * personalization work is never scheduled or started by a mount. */
export const VOICE_FEW_SHOT_MAX = 3
export const VOICE_FEW_SHOT_EXCERPT_MAX_CHARS = 800
export const VOICE_VALIDATION_POST_COUNT = 3
export const POST_TARGET_LENGTH_MIN = 100
export const POST_TARGET_LENGTH_MAX = 10_000

/** How long the HEIC decoder worker stays alive after its last file. Its WASM heap does
 *  not shrink after a 12 MP decode, so it is not kept for a whole session; the chunk is
 *  in the browser cache, so bringing it back for the next batch is cheap. */
export const HEIF_DECODER_IDLE_MS = 30_000

/** Voice display-name ceiling, mirrored from the voice context so the field can say so before
 *  the round trip; the server stays authoritative. Counted in Unicode scalar values, like the
 *  backend, so a Hangul syllable is one character here too. */
export const VOICE_NAME_MAX_CHARS = 50

/** Purpose (용도) brief ceilings, mirrored from `PURPOSE_*_MAX_CHARS` on the backend so the
 *  create/edit fields can count down before the round trip; the server stays authoritative.
 *  Counted in Unicode scalar values, like the backend, so a Hangul syllable is one character.
 *  A malformed or non-positive override falls back to the default rather than disabling the
 *  counter — a build-time typo must not silently remove the client-side bound. */
const positiveIntEnv = (raw: string | undefined, fallback: number): number => {
  const value = Number(raw)
  return Number.isInteger(value) && value > 0 ? value : fallback
}
export const PURPOSE_NAME_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_PURPOSE_NAME_MAX_CHARS,
  40,
)
export const PURPOSE_DESCRIPTION_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_PURPOSE_DESCRIPTION_MAX_CHARS,
  200,
)
export const PURPOSE_INSTRUCTIONS_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_PURPOSE_INSTRUCTIONS_MAX_CHARS,
  2000,
)

/** Writing-guideline (작문 지침) text ceiling, mirrored from `GUIDELINE_TEXT_MAX_CHARS` for the
 *  live counter; the server stays authoritative. The per-account cap is deliberately not
 *  mirrored — it is a prompt-size guard the backend owns, and the create form relays its
 *  refusal message instead of predicting it. */
export const GUIDELINE_TEXT_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_GUIDELINE_TEXT_MAX_CHARS,
  300,
)
