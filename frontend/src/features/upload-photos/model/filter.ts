import { UPLOAD_ALLOWED_EXTENSIONS, UPLOAD_MAX_FILE_MB } from '@/shared/config'
import { type DecodeFailure, fileExtension } from '@/shared/lib'

/** Why a selected file was never uploaded (PRD F-2: listed under "건너뜀" with a reason). */
export type SkipReason = 'extension' | 'too-large' | DecodeFailure

export type FileVerdict = { kind: 'accepted' } | { kind: 'skipped'; reason: SkipReason }

const ALLOWED = new Set<string>(UPLOAD_ALLOWED_EXTENSIONS)
const MAX_BYTES = UPLOAD_MAX_FILE_MB * 1024 * 1024

/** The gate at selection, before any bytes are read: extension and pre-conversion size. */
export function filterFile(file: { name: string; size: number }): FileVerdict {
  if (!ALLOWED.has(fileExtension(file.name))) return { kind: 'skipped', reason: 'extension' }
  if (file.size > MAX_BYTES) return { kind: 'skipped', reason: 'too-large' }
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
    case 'unreadable':
      return i18next.t('upload.skip.unreadable', { ns: 'posts' })
    case 'heif-unsupported':
      return i18next.t('upload.skip.heifUnsupported', { ns: 'posts' })
  }
}
import i18next from 'i18next'
