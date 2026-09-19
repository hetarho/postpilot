/** The photo pipeline (PRD F-2, §6.2). Every photo is decoded, downscaled and re-encoded
 *  in the browser before it is uploaded ([I6]); these are the knobs of that step, and the
 *  selection ceilings the picker refuses by. They moved out of `shared/config` with T258:
 *  only this feature reads them. `shared/config` owns the env READ, this file owns the
 *  limit and its default (ARCH-21). */
import { ENV_LIMIT_OVERRIDES, positiveIntEnv } from '@/shared/config'
/** Pre-conversion size cap, checked at selection. Advice to the user, not a security
 *  boundary — the server enforces its own cap on what actually lands. */
export const UPLOAD_MAX_FILE_MB = 25

/** The most photos one post may hold. It exists for the credit hold, not for storage:
 *  observation batches photos, so the model calls a generate job makes — and therefore the
 *  credits it reserves before starting — grow with the photo count. The server enforces the
 *  same ceiling; this copy is what lets the selection gate say so before a file is
 *  converted. */
export const UPLOAD_MAX_PHOTOS_PER_POST = 30

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
  ENV_LIMIT_OVERRIDES.uploadMaxVideosPerPost,
  3,
)
export const UPLOAD_MAX_VIDEO_MB = positiveIntEnv(ENV_LIMIT_OVERRIDES.uploadMaxVideoMB, 200)
export const VIDEO_MAX_SECONDS = positiveIntEnv(ENV_LIMIT_OVERRIDES.videoMaxSeconds, 60)

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
