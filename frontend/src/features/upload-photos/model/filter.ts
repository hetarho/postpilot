import {
  UPLOAD_ALLOWED_EXTENSIONS,
  UPLOAD_MAX_FILE_MB,
  UPLOAD_MAX_PHOTOS_PER_POST,
} from '@/shared/config'
import { type DecodeFailure, fileExtension } from '@/shared/lib'

/** Why a selected file was never uploaded (PRD F-2: listed under "건너뜀" with a reason). */
export type SkipReason = 'extension' | 'too-large' | 'too-many' | DecodeFailure

export type FileVerdict = { kind: 'accepted' } | { kind: 'skipped'; reason: SkipReason }

const ALLOWED = new Set<string>(UPLOAD_ALLOWED_EXTENSIONS)
const MAX_BYTES = UPLOAD_MAX_FILE_MB * 1024 * 1024

/** The gate at selection, before any bytes are read: extension, pre-conversion size, and
 *  the post's photo ceiling.
 *
 *  `alreadyHeld` is what the post already has plus what earlier files in this same pick have
 *  taken, so a single selection that would cross the ceiling is cut at the right file rather
 *  than accepted whole and refused one at a time by the server. */
export function filterFile(file: { name: string; size: number }, alreadyHeld = 0): FileVerdict {
  if (!ALLOWED.has(fileExtension(file.name))) return { kind: 'skipped', reason: 'extension' }
  if (file.size > MAX_BYTES) return { kind: 'skipped', reason: 'too-large' }
  if (alreadyHeld >= UPLOAD_MAX_PHOTOS_PER_POST) return { kind: 'skipped', reason: 'too-many' }
  return { kind: 'accepted' }
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
    case 'unreadable':
      return i18next.t('upload.skip.unreadable', { ns: 'posts' })
    case 'heif-unsupported':
      return i18next.t('upload.skip.heifUnsupported', { ns: 'posts' })
  }
}
import i18next from 'i18next'
