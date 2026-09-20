/** Build-time environment and cross-slice constants (ARCH-21).
 *
 *  What belongs here: a value read from `import.meta.env`, and a constant whose
 *  consumers span two domains. A product limit or a slice's own UI tuning does
 *  NOT — it is a named export of the owning slice's `config` segment, because one
 *  file that every slice edits is a merge hotspot (review/arch-260919 F4). */

/** Backend origin for built assets. In dev this is unused: Vite proxies '/api' to the
 *  backend (vite.config.ts), which also sidesteps CORS. */
export const API_URL = import.meta.env.VITE_API_URL ?? ''

/** Public OAuth client identifier. Empty is the supported feature-off state. */
export const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID?.trim() ?? ''

/** Public Toss Payments client key for the hosted card window. */
export const TOSS_CLIENT_KEY = import.meta.env.VITE_TOSS_CLIENT_KEY?.trim() ?? ''

/** The raw `VITE_*` mirrors of backend ceilings. This module owns the env READ; the
 *  owning slice names the limit and supplies its default (ARCH-21), the way the backend's
 *  `platform/config` parses env and the context ctor merges it. */
export const ENV_LIMIT_OVERRIDES = {
  uploadMaxVideosPerPost: import.meta.env.VITE_UPLOAD_MAX_VIDEOS_PER_POST,
  uploadMaxVideoMB: import.meta.env.VITE_UPLOAD_MAX_VIDEO_MB,
  videoMaxSeconds: import.meta.env.VITE_VIDEO_MAX_SECONDS,
  templateNameMaxChars: import.meta.env.VITE_TEMPLATE_NAME_MAX_CHARS,
  templateDescriptionMaxChars: import.meta.env.VITE_TEMPLATE_DESCRIPTION_MAX_CHARS,
  templateBodyMaxChars: import.meta.env.VITE_TEMPLATE_BODY_MAX_CHARS,
  templatePhotoRowMax: import.meta.env.VITE_TEMPLATE_PHOTO_ROW_MAX,
  templateAskLabelMaxChars: import.meta.env.VITE_TEMPLATE_ASK_LABEL_MAX_CHARS,
  templateAskValueMaxChars: import.meta.env.VITE_TEMPLATE_ASK_VALUE_MAX_CHARS,
  templateAskMaxPerBody: import.meta.env.VITE_TEMPLATE_ASK_MAX_PER_BODY,
  guidelineTextMaxChars: import.meta.env.VITE_GUIDELINE_TEXT_MAX_CHARS,
  memoryTextMaxChars: import.meta.env.VITE_MEMORY_TEXT_MAX_CHARS,
  memoryTagsMax: import.meta.env.VITE_MEMORY_TAGS_MAX,
  memoryMaxPerAccount: import.meta.env.VITE_MEMORY_MAX_PER_ACCOUNT,
  memoryInjectMax: import.meta.env.VITE_MEMORY_INJECT_MAX,
} as const

/** Reads a `VITE_*` mirror of a backend ceiling. A malformed or non-positive override falls
 *  back to the default rather than disabling the bound — a build-time typo must not silently
 *  remove a client-side check. */
export function positiveIntEnv(raw: string | undefined, fallback: number): number {
  const value = Number(raw)
  return Number.isInteger(value) && value > 0 ? value : fallback
}

/** Theme is a browser-local interface preference, never deployment or account state. */
export const THEME_PREFERENCE_STORAGE_KEY = 'postpilot.theme' as const
export const DEFAULT_THEME_PREFERENCE = 'system' as const

/** How long the editor waits after the last keystroke before saving the draft (PRD F-2:
 *  the memo has to survive a phone that kills the tab). Short enough that a force-quit
 *  costs at most a second of typing, long enough that a fast typist is not one request
 *  per character. Cross-slice: three save features share one queue's tuning. */
export const AUTOSAVE_DEBOUNCE_MS = 1_000

/** First delay before retrying a failed autosave; doubled per attempt up to the cap.
 *  The cap is what keeps a tab left open on a dead network from turning into a request
 *  loop while still recovering on its own once the network returns. */
export const AUTOSAVE_RETRY_BASE_MS = 1_000
export const AUTOSAVE_RETRY_MAX_MS = 30_000

/** How long 저장됨 stays on the editor's status line after a completed save before the line goes
 *  quiet. The queue's own `saved` is a STANDING state — it holds for the life of a queue that has
 *  ever saved — so without a settle the line permanently claims a save the user made minutes ago,
 *  and the post's status can never reach the one line that reports it. Long enough to be read as
 *  the answer to the keystroke that caused it, short enough that a draft left alone says nothing. */
export const SAVE_STATUS_SETTLED_MS = 2_000

/** Generation jobs are durable on the server; polling only observes their state. Every
 *  domain that waits on a job reads this one interval. */
export const POLL_INTERVAL_MS = 2_000

/** How long successful clipboard feedback remains visible. */
export const COPY_FEEDBACK_MS = 1_500

/** The app-wide TanStack Query defaults, applied in app/providers/query-client. They are a
 *  conservative floor: any query with its own freshness or failure policy overrides them at
 *  its call site. The stale window is deliberately twice the session's own — the session is
 *  the one query whose staleness gates navigation, so it re-checks while ordinary data is
 *  still trusted. One retry covers a dropped connection without turning a genuine server
 *  refusal into a multi-second wait. */
export const QUERY_STALE_MS = 60_000
export const QUERY_RETRY_COUNT = 1
