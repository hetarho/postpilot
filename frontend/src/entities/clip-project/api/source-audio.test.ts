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
  it('preserves optional capture time and leaves older source metadata unchanged', () => {
    const batch = toClipSourceBatch(
      create(ClipSourceBatchSchema, {
        id: 'batch',
        projectId: 'project',
        state: 'ready',
        sources: [
          source,
          {
            ...source,
            id: 'captured',
            metadata: { ...source.metadata, capturedAt: '2026-10-15T03:34:56Z' },
          },
        ],
      }),
    )
    expect(batch.sources[0].metadata).not.toHaveProperty('capturedAt')
    expect(batch.sources[1].metadata.capturedAt).toBe('2026-10-15T03:34:56Z')
  })
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
