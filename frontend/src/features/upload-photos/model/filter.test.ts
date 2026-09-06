import { describe, expect, it } from 'vitest'
import {
  UPLOAD_ALLOWED_EXTENSIONS,
  UPLOAD_MAX_FILE_MB,
  UPLOAD_MAX_VIDEO_MB,
  UPLOAD_MAX_VIDEOS_PER_POST,
  UPLOAD_VIDEO_EXTENSIONS,
  VIDEO_MAX_SECONDS,
} from '@/shared/config'
import { filterFile, skipReasonLabel } from './filter'

const MB = 1024 * 1024
const none = { photos: 0, videos: 0 }

describe('filterFile', () => {
  it.each(UPLOAD_ALLOWED_EXTENSIONS)('accepts .%s, in either case', (extension) => {
    expect(filterFile({ name: `IMG_1.${extension}`, size: 3 * MB }, none)).toEqual({
      kind: 'accepted',
      attachment: 'photo',
    })
    expect(filterFile({ name: `IMG_1.${extension.toUpperCase()}`, size: 3 * MB }, none)).toEqual({
      kind: 'accepted',
      attachment: 'photo',
    })
  })

  // VIDEO-7: one picker takes both kinds, and the extension is what sorts them.
  it.each(UPLOAD_VIDEO_EXTENSIONS)('accepts .%s as a video, in either case', (extension) => {
    expect(filterFile({ name: `clip.${extension}`, size: 20 * MB }, none)).toEqual({
      kind: 'accepted',
      attachment: 'video',
    })
    expect(filterFile({ name: `clip.${extension.toUpperCase()}`, size: 20 * MB }, none)).toEqual({
      kind: 'accepted',
      attachment: 'video',
    })
  })

  // A1 (plan AC2): an executable, or anything not on either list, never uploads.
  it('skips an executable and any extension not on the list, with the reason', () => {
    expect(filterFile({ name: 'setup.exe', size: 10 }, none)).toEqual({
      kind: 'skipped',
      reason: 'extension',
    })
    expect(filterFile({ name: 'clip.avi', size: 10 }, none)).toEqual({
      kind: 'skipped',
      reason: 'extension',
    })
    expect(filterFile({ name: 'noextension', size: 10 }, none)).toEqual({
      kind: 'skipped',
      reason: 'extension',
    })
  })

  // A3 (plan AC4): the cap is on the original, at selection.
  it('skips a file over the cap and accepts one under it', () => {
    expect(filterFile({ name: 'big.heic', size: (UPLOAD_MAX_FILE_MB + 1) * MB }, none)).toEqual({
      kind: 'skipped',
      reason: 'too-large',
    })
    expect(filterFile({ name: 'ok.heic', size: 20 * MB }, none)).toEqual({
      kind: 'accepted',
      attachment: 'photo',
    })
  })

  // Nothing converts a clip, so its size at selection is its size on the server (VIDEO-4).
  it('skips a video over its own cap, which is not the photo cap', () => {
    expect(filterFile({ name: 'big.mp4', size: (UPLOAD_MAX_VIDEO_MB + 1) * MB }, none)).toEqual({
      kind: 'skipped',
      reason: 'video-too-large',
    })
    // Far over the PHOTO cap and still fine: the two ceilings are different by an order of
    // magnitude, and a clip is never measured against the photo one.
    expect(filterFile({ name: 'ok.mp4', size: (UPLOAD_MAX_FILE_MB + 10) * MB }, none)).toEqual({
      kind: 'accepted',
      attachment: 'video',
    })
  })

  // The two counts are separate: a post full of photos may still take a clip, and the other
  // way round.
  it('counts each kind against its own ceiling', () => {
    const full = { photos: 0, videos: UPLOAD_MAX_VIDEOS_PER_POST }
    expect(filterFile({ name: 'clip.mp4', size: MB }, full)).toEqual({
      kind: 'skipped',
      reason: 'video-too-many',
    })
    expect(filterFile({ name: 'IMG_1.jpg', size: MB }, full)).toEqual({
      kind: 'accepted',
      attachment: 'photo',
    })
    const manyPhotos = { photos: 30, videos: 0 }
    expect(filterFile({ name: 'IMG_1.jpg', size: MB }, manyPhotos)).toEqual({
      kind: 'skipped',
      reason: 'too-many',
    })
    expect(filterFile({ name: 'clip.mp4', size: MB }, manyPhotos)).toEqual({
      kind: 'accepted',
      attachment: 'video',
    })
  })

  it('has a Korean label for every reason', () => {
    expect(skipReasonLabel('extension')).toContain('heic')
    expect(skipReasonLabel('too-large')).toContain(`${UPLOAD_MAX_FILE_MB}MB`)
    expect(skipReasonLabel('unreadable')).not.toBe('')
    expect(skipReasonLabel('heif-unsupported')).toContain('HEIC')
    expect(skipReasonLabel('video-too-large')).toContain(`${UPLOAD_MAX_VIDEO_MB}MB`)
    expect(skipReasonLabel('video-too-many')).toContain(`${UPLOAD_MAX_VIDEOS_PER_POST}`)
    expect(skipReasonLabel('video-too-long')).toContain(`${VIDEO_MAX_SECONDS}`)
    expect(skipReasonLabel('video-unreadable')).not.toBe('')
  })
})
