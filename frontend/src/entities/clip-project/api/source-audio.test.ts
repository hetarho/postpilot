import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ClipSourceBatchSchema } from '@/shared/api'
import { toClipSourceBatch } from './clip-project'

describe('owner source sound on the source projection', () => {
  const source = {
    id: 'source',
    state: 'ready',
    actualBytes: 100n,
    metadata: {
      filename: 'a.mp4',
      contentType: 'video/mp4',
      bytes: 100n,
      durationMs: 10000,
      width: 1920,
      height: 1080,
      fingerprint: 'a'.repeat(64),
    },
  }
  it('carries the choice and defaults a source that predates it to off', () => {
    const on = toClipSourceBatch(
      create(ClipSourceBatchSchema, {
        id: 'batch',
        projectId: 'project',
        state: 'ready',
        expiresAt: '2099-01-01T00:00:00Z',
        sources: [{ ...source, retainOriginalAudio: true }],
      }),
    )
    expect(on.sources[0]!.retainOriginalAudio).toBe(true)
    const off = toClipSourceBatch(
      create(ClipSourceBatchSchema, {
        id: 'batch',
        projectId: 'project',
        state: 'ready',
        expiresAt: '2099-01-01T00:00:00Z',
        sources: [source],
      }),
    )
    expect(off.sources[0]!.retainOriginalAudio).toBe(false)
  })
})
