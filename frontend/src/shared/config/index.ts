/** Build-time environment. VITE_* values are baked into the bundle, so only ever
 *  public configuration belongs here — never a secret. */

/** Backend origin for built assets. In dev this is unused: Vite proxies '/api' to the
 *  backend (vite.config.ts), which also sidesteps CORS. */
export const API_URL = import.meta.env.VITE_API_URL ?? ''

/** Public OAuth client identifier. Empty is the supported feature-off state. */
export const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID?.trim() ?? ''

/** Public Toss Payments client key for the hosted card window. */
export const TOSS_CLIENT_KEY = import.meta.env.VITE_TOSS_CLIENT_KEY?.trim() ?? ''

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

/** How much of a voice profile version's generation snapshot the version preview shows before
 *  the reader asks for the rest. A snapshot is a whole post, and the version list is a list: the
 *  point of the preview is to recognize a voice's register in a few sentences, which is roughly
 *  a phone screen's worth of text. */
export const VOICE_VERSION_PREVIEW_CHARS = 400

/** How long 저장됨 stays on the editor's status line after a completed save before the line goes
 *  quiet. The queue's own `saved` is a STANDING state — it holds for the life of a queue that has
 *  ever saved — so without a settle the line permanently claims a save the user made minutes ago,
 *  and the post's status can never reach the one line that reports it. Long enough to be read as
 *  the answer to the keystroke that caused it, short enough that a draft left alone says nothing. */
export const SAVE_STATUS_SETTLED_MS = 2_000

/** How long the editor's cross-route handoff may sit unread
 *  (`pages/editor/model/editor-handoff.ts`). It is meant to be picked up a tick later by
 *  the editor the mint navigation mounts; anything older belongs to a navigation that
 *  never happened, and applying it would put stale text back into a post that has since
 *  moved on. */
export const EDITOR_HANDOFF_TTL_MS = 5_000

/** Reads a `VITE_*` mirror of a backend ceiling. A malformed or non-positive override falls
 *  back to the default rather than disabling the bound — a build-time typo must not silently
 *  remove a client-side check.
 *
 *  A function declaration, not a const arrow: the mirrors below are declared in the order the
 *  file reads best, and several of them sit above this line. */
function positiveIntEnv(raw: string | undefined, fallback: number): number {
  const value = Number(raw)
  return Number.isInteger(value) && value > 0 ? value : fallback
}

/** The photo pipeline (PRD F-2, §6.2). Every photo is decoded, downscaled and re-encoded
 *  in the browser before it is uploaded ([I6]); these are the knobs of that step. */

/** Pre-conversion size cap, checked at selection. Advice to the user, not a security
 *  boundary — the server enforces its own cap on what actually lands. */
export const UPLOAD_MAX_FILE_MB = 25

/** The most photos one post may hold. It exists for the credit hold, not for storage:
 *  observation batches photos, so the model calls a generate job makes — and therefore the
 *  credits it reserves before starting — grow with the photo count. The server enforces the
 *  same ceiling; this copy is what lets the selection gate say so before a file is
 *  converted. */
export const UPLOAD_MAX_PHOTOS_PER_POST = 30

/** T071 admission mirrors. The API and media probe remain authoritative. */
export const CLIP_SOURCE_MAX_COUNT = 20
export const CLIP_SOURCE_MAX_FILENAME_CHARS = 255
export const CLIP_SOURCE_MAX_DURATION_MS = 30 * 60 * 1000
export const CLIP_SOURCE_MAX_FILE_BYTES = 2 * 1024 * 1024 * 1024
export const CLIP_SOURCE_MAX_BATCH_BYTES = 8 * 1024 * 1024 * 1024
export const CLIP_SOURCE_BATCH_TTL_MS = 6 * 60 * 60 * 1000
export const CLIP_SOURCE_FINGERPRINT_CHUNK_BYTES = 64 * 1024
export const CLIP_SOURCE_CONTAINERS: Readonly<Record<string, readonly string[]>> = {
  mp4: ['video/mp4'],
  mov: ['video/quicktime'],
  m4v: ['video/x-m4v', 'video/mp4'],
  webm: ['video/webm'],
}

/** Compared case-insensitively against the extension of the selected file. Anything else
 *  is listed as skipped, never uploaded. */
export const UPLOAD_ALLOWED_EXTENSIONS = ['jpg', 'jpeg', 'png', 'webp', 'heic', 'heif'] as const

/** Videos (VIDEO-3). Unlike a photo, a clip is uploaded EXACTLY as picked — nothing here
 *  decodes, downscales or transcodes it (VIDEO-4) — so these ceilings are the whole of what
 *  the browser can do about size, and the server enforces the same three on confirm.
 *
 *  All three are mirrored from the backend's own values rather than chosen here: raising one
 *  on this side alone would let the picker accept a file the save refuses. The three together
 *  are what keeps one observation call inside the credit hold's prompt assumption (QUOTA-14).
 */
export const UPLOAD_MAX_VIDEOS_PER_POST = positiveIntEnv(
  import.meta.env.VITE_UPLOAD_MAX_VIDEOS_PER_POST,
  3,
)
export const UPLOAD_MAX_VIDEO_MB = positiveIntEnv(import.meta.env.VITE_UPLOAD_MAX_VIDEO_MB, 200)
export const VIDEO_MAX_SECONDS = positiveIntEnv(import.meta.env.VITE_VIDEO_MAX_SECONDS, 60)

/** The containers a clip may arrive in, compared case-insensitively against the extension.
 *  They are the same four the server signs a PUT for; anything else is listed as skipped. */
export const UPLOAD_VIDEO_EXTENSIONS = ['mp4', 'mov', 'm4v', 'webm'] as const

/** The long edge a photo is downscaled to. Smaller images are never upscaled. */
export const IMAGE_MAX_LONG_EDGE_PX = 1024

/** JPEG encoder quality (0–1) for the uploaded copy. */
export const IMAGE_JPEG_QUALITY = 0.85

/** How many photos may be decoding/resizing at once. Each decoded phone photo is tens of
 *  megabytes of pixels until it is downscaled, so eight at once would push a phone into
 *  swapping; one at a time leaves the browser's own parallel JPEG decoder idle. */
export const UPLOAD_CONVERT_CONCURRENCY = 2

/** How long the model catalog is trusted before it is re-asked. The usable-model list only
 *  changes when an operator curates it, so refetching it on every mount buys nothing; a few
 *  minutes means a newly enabled model shows up without a reload while the dropdowns stay
 *  instant. */
export const MODEL_CATALOG_STALE_MS = 5 * 60_000

/** How long a credit balance is trusted before it is re-asked. The shell's credit control
 *  and the account popover read one cache entry, so a window is what keeps opening the
 *  popover from costing a second request; a generation takes far longer than this, so the
 *  figure is never stale by the time a job has actually moved it. */
export const PLAN_BALANCE_STALE_MS = 30_000

/** What the plan comparison's estimator starts from, and how far each slider goes.
 *
 *  The ceilings are the product's own: a post holds at most `UPLOAD_MAX_PHOTOS_PER_POST`
 *  photos and `UPLOAD_MAX_VIDEOS_PER_POST` clips, so an estimate outside them would price a
 *  post the product refuses to accept. The character range is editorial — a short review to a
 *  long one — and the default is the shape most posts in this product actually have. */
export const PLAN_ESTIMATE_BOUNDS = {
  chars: { min: 200, max: 5_000, step: 100 },
  photos: { min: 0, max: UPLOAD_MAX_PHOTOS_PER_POST, step: 1 },
  videos: { min: 0, max: UPLOAD_MAX_VIDEOS_PER_POST, step: 1 },
} as const

export const PLAN_ESTIMATE_DEFAULTS = { chars: 1_000, photos: 5, videos: 0 } as const

/** Where the reader's own case is kept between visits. Namespaced like the theme's key. */
export const PLAN_ESTIMATE_STORAGE_KEY = 'postpilot.plan-estimate'

/** How far apart the plan ladder's rungs arrive, in milliseconds. Each rung's rise animation
 *  starts this much after the one before it, so four cards read as one ladder unfolding rather
 *  than four things appearing at once — and short enough that the last rung is still up before
 *  a reader has finished the first. */
export const PROMO_RISE_STAGGER_MS = 60

/** How long a plan card's post count takes to climb to a new value as a slider moves. Long
 *  enough to read as a count rather than a flicker, short enough to have settled before the
 *  thumb reaches the next slider stop. */
export const PROMO_COUNT_UP_MS = 480

/** How far a promotional card tilts toward a fine pointer, in degrees at the card's edge, and
 *  the perspective distance the tilt is seen from. Seven degrees is where a card still reads as
 *  a card answering the hand rather than a panel falling over; a shorter perspective makes the
 *  same angle look like a cheap trick. */
export const PROMO_TILT_MAX_DEG = 7
export const PROMO_TILT_PERSPECTIVE_PX = 900

/** The aurora shader's frame budget. Thirty frames a second is more than a slow drift needs and
 *  half what a display will ask for, so the stage costs battery like a video, not like a game. */
export const PROMO_AURORA_MIN_FRAME_MS = 33
/** The shader renders at this fraction of the stage's CSS pixels: the picture is smooth noise
 *  the browser scales up, so full resolution would buy nothing visible for four times the work. */
export const PROMO_AURORA_RESOLUTION_SCALE = 0.5

/** The provider slugs the operator's catalog screen lifts to the top, in this order; every
 *  other vendor follows alphabetically.
 *
 *  The provider's catalog is ~420 models across ~40 vendors, so an alphabetical list buries
 *  the handful anyone actually reaches for behind two screens of scrolling. The order is
 *  editorial — the vendors whose models are worth exposing on quality or price — and it is a
 *  display preference only: nothing here grants access, and the search and filters reach
 *  every vendor either way. */
export const FEATURED_MODEL_PROVIDERS: readonly string[] = [
  'openai',
  'anthropic',
  'google',
  'deepseek',
  'z-ai',
  'minimax',
  'meta-llama',
  'meta',
  'x-ai',
  'qwen',
  'moonshotai',
  'mistralai',
]

/** The five purposes a model can be registered to, in the order the operator's tabs show
 *  them. The slugs are the wire/DB contract (change 20); the first three feed the user-facing
 *  observe/analyze/write stages, the two generation purposes are settings for features that
 *  do not exist yet. Labels live in the i18n resources under `models:catalog.purposeTab`. */
export const MODEL_PURPOSES = [
  'photo-analysis',
  'style-analysis',
  'writing',
  'image-generation',
  'video-generation',
] as const

export type ModelPurpose = (typeof MODEL_PURPOSES)[number]

/** The height the operator's catalog list assumes for a row it has not measured yet. Only the
 *  scrollbar depends on it: every mounted row reports its real height, which differs a lot
 *  between a plain candidate and an enabled model carrying two controls. */
export const CATALOG_ROW_ESTIMATE_PX = 132

/** How many catalog rows are mounted beyond the viewport. Enough that a fast flick does not
 *  reach blank space before the next row renders, few enough that the mounted subtree stays
 *  small — which is the whole point of virtualizing a several-hundred-row list. */
export const CATALOG_ROW_OVERSCAN = 6

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

/** The clear space an open listbox panel keeps from the top or bottom edge of the viewport. It
 *  matches the popover's gutter for the same reason: a bounded overlay should stop where the
 *  content column stops rather than run to the glass. */
export const LISTBOX_VIEWPORT_GUTTER_PX = 16

/** The `mt-1` an open listbox panel keeps between itself and its trigger, as a number, so the
 *  height measurement can subtract the gap the CSS is about to add. */
export const LISTBOX_TRIGGER_GAP_PX = 4

/** The shortest an open listbox panel is squeezed to before it stops honouring the room it
 *  measured — roughly three option rows. A trigger closer than this to BOTH edges is a layout
 *  bug at the call site, and overhanging is the better failure there. */
export const LISTBOX_MIN_PANEL_PX = 132

/** How much of the viewport an open listbox may take when there is more room than it needs. A
 *  forty-model catalog would otherwise open a panel as tall as the screen from a field near the
 *  top of a desktop page, which buries the field it belongs to. */
export const LISTBOX_MAX_VIEWPORT_RATIO = 0.5

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

/** What 목표 글자 수 사용 fills the empty field with. A ticked checkbox over a blank number input is
 *  an invalid form the user did not ask for — the field renders its range error before anyone has
 *  typed a character — so the box arrives with a usable value already in it, roughly the length of
 *  an ordinary blog post. It is a STARTING POINT, not a floor: the field stays free between
 *  POST_TARGET_LENGTH_MIN and POST_TARGET_LENGTH_MAX, and a value already typed is never
 *  overwritten by it. */
export const POST_TARGET_LENGTH_DEFAULT = 1_000

/** How many tags a run asks for (POST-63). Unlike the length there is no "natural" count to
 *  opt into: a post never saved with one reads as the default, and the field is always shown.
 *  The server owns the same three values (`config.PostTagCount*`) and refuses anything outside
 *  the range with POST_TAG_COUNT_INVALID. */
export const POST_TAG_COUNT_DEFAULT = 4
export const POST_TAG_COUNT_MIN = 1
export const POST_TAG_COUNT_MAX = 10

/** How long the HEIC decoder worker stays alive after its last file. Its WASM heap does
 *  not shrink after a 12 MP decode, so it is not kept for a whole session; the chunk is
 *  in the browser cache, so bringing it back for the next batch is cheap. */
export const HEIF_DECODER_IDLE_MS = 30_000

/** Voice display-name ceiling, mirrored from the voice context so the field can say so before
 *  the round trip; the server stays authoritative. Counted in Unicode scalar values, like the
 *  backend, so a Hangul syllable is one character here too. */
export const VOICE_NAME_MAX_CHARS = 50

/** The optional 말투 설명 ceiling, mirrored from `VoiceDescriptionMaxChars` the same way. It is
 *  far below a sample's length on purpose: this field states the register the user wants, it
 *  does not demonstrate it. */
export const VOICE_DESCRIPTION_MAX_CHARS = 500

/** Template (템플릿) field ceilings, mirrored from `TEMPLATE_*_MAX_CHARS` on the backend so the
 *  create/edit fields can count down before the round trip; the server stays authoritative.
 *  Counted in Unicode scalar values, like the backend, so a Hangul syllable is one character.
 *  A malformed or non-positive override falls back to the default rather than disabling the
 *  counter — a build-time typo must not silently remove the client-side bound.
 *
 *  The per-account cap and the repeat-expansion bound are deliberately NOT mirrored: both are
 *  server-owned guards, and the second depends on the post's photo count rather than on the
 *  template being edited. */
export const TEMPLATE_NAME_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_TEMPLATE_NAME_MAX_CHARS,
  40,
)
export const TEMPLATE_DESCRIPTION_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_TEMPLATE_DESCRIPTION_MAX_CHARS,
  200,
)
export const TEMPLATE_BODY_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_TEMPLATE_BODY_MAX_CHARS,
  4000,
)

/** How many photos one photo position may place side by side, mirrored from
 *  `TEMPLATE_PHOTO_ROW_MAX`. Unlike the ceilings above this one is not only a counter: the
 *  builder's stepper cannot offer a value the server's parser would refuse on save, so the two
 *  numbers have to be raised together. */
export const TEMPLATE_PHOTO_ROW_MAX = positiveIntEnv(import.meta.env.VITE_TEMPLATE_PHOTO_ROW_MAX, 4)

/** The data-field ceilings, mirrored from `TEMPLATE_ASK_*` (TEMPLATE-43): a field's title, one
 *  answer's text, and how many fields one body may declare. The first two are live counters and
 *  the third is a refusal the builder states before the server has to; the backend stays
 *  authoritative on all three. */
export const TEMPLATE_ASK_LABEL_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_TEMPLATE_ASK_LABEL_MAX_CHARS,
  40,
)
export const TEMPLATE_ASK_VALUE_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_TEMPLATE_ASK_VALUE_MAX_CHARS,
  500,
)
export const TEMPLATE_ASK_MAX_PER_BODY = positiveIntEnv(
  import.meta.env.VITE_TEMPLATE_ASK_MAX_PER_BODY,
  10,
)

/** Writing-guideline (작문 지침) text ceiling, mirrored from `GUIDELINE_TEXT_MAX_CHARS` for the
 *  live counter; the server stays authoritative. The per-account cap is deliberately not
 *  mirrored — it is a prompt-size guard the backend owns, and the create form relays its
 *  refusal message instead of predicting it. */
export const GUIDELINE_TEXT_MAX_CHARS = positiveIntEnv(
  import.meta.env.VITE_GUIDELINE_TEXT_MAX_CHARS,
  300,
)

/** The clip design system: the numbers the renderer embeds, read rather than
 *  copied. See shared/config/clip-design.ts. */
export {
  CLIP_DESIGN,
  CLIP_STYLES,
  CLIP_PRESETS,
  CLIP_DISCLOSURES,
  CLIP_CTAS,
  CLIP_SHADOW,
  CLIP_SPACING,
  CLIP_TIMING,
  CLIP_TRANSITION,
  CLIP_COPY,
  CLIP_RAPID,
  CLIP_GUARDS,
  CLIP_CLASSES,
  CLIP_SCENE_STYLES,
  CLIP_FACTS,
  CLIP_ACCENT_HEX,
  CLIP_TYPE,
  CLIP_VOICE,
  clipStyle,
  clipType,
  clipPaint,
} from './clip-design'
export type {
  ClipStyleId,
  ClipCaptionPace,
  ClipRatioId,
  ClipPresetId,
  ClipDisclosureId,
  ClipCTAId,
} from './clip-design'
export { CLIP_COMPOSITION_LIMITS } from './clip-composition'
