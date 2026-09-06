import {
  UPLOAD_ALLOWED_EXTENSIONS,
  UPLOAD_MAX_FILE_MB,
  UPLOAD_MAX_PHOTOS_PER_POST,
  UPLOAD_MAX_VIDEO_MB,
  UPLOAD_MAX_VIDEOS_PER_POST,
  UPLOAD_VIDEO_EXTENSIONS,
  VIDEO_MAX_SECONDS,
} from '@/shared/config'
import { type DecodeFailure, fileExtension } from '@/shared/lib'

/** Why a selected file was never uploaded (PRD F-2: listed under "건너뜀" with a reason).
 *
 *  The video reasons are separate from the photo ones because the numbers differ by an order
 *  of magnitude and the user is told which ceiling they met. `video-too-long` is the one
 *  verdict this module cannot reach on its own: a duration is only known after the container's
 *  metadata is read, so the upload path applies it (VIDEO-3). */
export type SkipReason =
  | 'extension'
  | 'too-large'
  | 'too-many'
  | 'video-too-large'
  | 'video-too-many'
  | 'video-too-long'
  | 'video-unreadable'
  | DecodeFailure

/** Which kind of attachment an accepted file becomes. One picker and one drop zone take both
 *  (VIDEO-7); the extension is what sorts them. */
export type AttachmentKind = 'photo' | 'video'

export type FileVerdict =
  { kind: 'accepted'; attachment: AttachmentKind } | { kind: 'skipped'; reason: SkipReason }

/** What the post already holds, plus what earlier files in this same pick have taken. The two
 *  ceilings are counted separately: a post full of photos may still take a clip. */
export interface HeldAttachments {
  photos: number
  videos: number
}

const ALLOWED = new Set<string>(UPLOAD_ALLOWED_EXTENSIONS)
const VIDEO_ALLOWED = new Set<string>(UPLOAD_VIDEO_EXTENSIONS)
const MAX_BYTES = UPLOAD_MAX_FILE_MB * 1024 * 1024
const MAX_VIDEO_BYTES = UPLOAD_MAX_VIDEO_MB * 1024 * 1024

/** The gate at selection, before any bytes are read: extension, pre-conversion size, and
 *  the post's photo ceiling.
 *
 *  `alreadyHeld` is what the post already has plus what earlier files in this same pick have
 *  taken, so a single selection that would cross the ceiling is cut at the right file rather
 *  than accepted whole and refused one at a time by the server. */
export function filterFile(
  file: { name: string; size: number },
  held: HeldAttachments = { photos: 0, videos: 0 },
): FileVerdict {
  const extension = fileExtension(file.name)
  if (VIDEO_ALLOWED.has(extension)) {
    // Nothing converts a clip, so its size at selection is its size on the server.
    if (file.size > MAX_VIDEO_BYTES) return { kind: 'skipped', reason: 'video-too-large' }
    if (held.videos >= UPLOAD_MAX_VIDEOS_PER_POST) {
      return { kind: 'skipped', reason: 'video-too-many' }
    }
    return { kind: 'accepted', attachment: 'video' }
  }
  if (!ALLOWED.has(extension)) return { kind: 'skipped', reason: 'extension' }
  if (file.size > MAX_BYTES) return { kind: 'skipped', reason: 'too-large' }
  if (held.photos >= UPLOAD_MAX_PHOTOS_PER_POST) return { kind: 'skipped', reason: 'too-many' }
  return { kind: 'accepted', attachment: 'photo' }
}

export function skipReasonLabel(reason: SkipReason): string {
  switch (reason) {
    case 'extension':
      return i18next.t('upload.skip.extension', {
        ns: 'posts',
        extensions: UPLOAD_ALLOWED_EXTENSIONS.join(', '),
      })
    case 'too-large':
      return i18next.t('upload.skip.tooLarge', { ns: 'posts', max: UPLOAD_MAX_FILE_MB })
    case 'too-many':
      return i18next.t('upload.skip.tooMany', { ns: 'posts', max: UPLOAD_MAX_PHOTOS_PER_POST })
    case 'video-too-large':
      return i18next.t('upload.skip.videoTooLarge', { ns: 'posts', max: UPLOAD_MAX_VIDEO_MB })
    case 'video-too-many':
      return i18next.t('upload.skip.videoTooMany', { ns: 'posts', max: UPLOAD_MAX_VIDEOS_PER_POST })
    case 'video-too-long':
      return i18next.t('upload.skip.videoTooLong', { ns: 'posts', max: VIDEO_MAX_SECONDS })
    case 'video-unreadable':
      return i18next.t('upload.skip.videoUnreadable', { ns: 'posts' })
    case 'unreadable':
      return i18next.t('upload.skip.unreadable', { ns: 'posts' })
    case 'heif-unsupported':
      return i18next.t('upload.skip.heifUnsupported', { ns: 'posts' })
  }
}
import i18next from 'i18next'
