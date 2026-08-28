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

const SKIP_LABELS: Record<SkipReason, string> = {
  extension: `사진 파일이 아니에요 (${UPLOAD_ALLOWED_EXTENSIONS.join(', ')}만 올릴 수 있어요)`,
  'too-large': `${UPLOAD_MAX_FILE_MB}MB를 넘어요`,
  unreadable: '사진을 읽을 수 없어요',
  'heif-unsupported': '이 기기에서는 HEIC를 변환할 수 없어요',
}

export function skipReasonLabel(reason: SkipReason): string {
  return SKIP_LABELS[reason]
}
