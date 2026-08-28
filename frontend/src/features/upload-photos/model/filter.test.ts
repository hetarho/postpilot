import { describe, expect, it } from 'vitest'
import { UPLOAD_ALLOWED_EXTENSIONS, UPLOAD_MAX_FILE_MB } from '@/shared/config'
import { filterFile, skipReasonLabel } from './filter'

const MB = 1024 * 1024

describe('filterFile', () => {
  it.each(UPLOAD_ALLOWED_EXTENSIONS)('accepts .%s, in either case', (extension) => {
    expect(filterFile({ name: `IMG_1.${extension}`, size: 3 * MB })).toEqual({ kind: 'accepted' })
    expect(filterFile({ name: `IMG_1.${extension.toUpperCase()}`, size: 3 * MB })).toEqual({
      kind: 'accepted',
    })
  })

  // A1 (plan AC2): an executable, or anything not on the list, never uploads.
  it('skips an executable and any extension not on the list, with the reason', () => {
    expect(filterFile({ name: 'setup.exe', size: 10 })).toEqual({ kind: 'skipped', reason: 'extension' })
    expect(filterFile({ name: 'clip.mov', size: 10 })).toEqual({ kind: 'skipped', reason: 'extension' })
    expect(filterFile({ name: 'noextension', size: 10 })).toEqual({ kind: 'skipped', reason: 'extension' })
  })

  // A3 (plan AC4): the cap is on the original, at selection.
  it('skips a file over the cap and accepts one under it', () => {
    expect(filterFile({ name: 'big.heic', size: (UPLOAD_MAX_FILE_MB + 1) * MB })).toEqual({
      kind: 'skipped',
      reason: 'too-large',
    })
    expect(filterFile({ name: 'ok.heic', size: 20 * MB })).toEqual({ kind: 'accepted' })
  })

  it('has a Korean label for every reason', () => {
    expect(skipReasonLabel('extension')).toContain('heic')
    expect(skipReasonLabel('too-large')).toContain(`${UPLOAD_MAX_FILE_MB}MB`)
    expect(skipReasonLabel('unreadable')).not.toBe('')
    expect(skipReasonLabel('heif-unsupported')).toContain('HEIC')
  })
})
