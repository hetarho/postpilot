import { describe, expect, it } from 'vitest'
import { createFakeAuthBackend } from '@/test/session'
import { createClipSourcePipeline } from './pipeline'

describe('clip source Connect boundary', () => {
  it('projects metadata only even when the caller carries local files or object URLs', async () => {
    const sourceRequests: unknown[] = []
    const { transport } = createFakeAuthBackend({
      user: { id: 'alice' },
      clips: {
        sourceRequests,
        projects: [
          {
            id: 'project',
            title: 'clip',
            videoTemplateId: 'template',
            ratio: 'vertical',
            targetDurationMs: 30000,
            disclosure: 'ad',
            cta: '',
            answers: [],
          },
        ],
      },
    })
    const pipeline = createClipSourcePipeline(transport)
    const manifest = [
      {
        filename: 'clip.mp4',
        contentType: 'video/mp4',
        bytes: 4,
        durationMs: 1000,
        width: 1920,
        height: 1080,
        fingerprint: 'a'.repeat(64),
        file: new File(['clip'], 'clip.mp4'),
        previewURL: 'blob:local',
      },
    ]
    const reservation = await pipeline.reserve('project', manifest)
    expect(reservation.uploads[0]?.headers).toEqual({
      'Content-Type': 'video/mp4',
      'If-None-Match': '*',
    })
    expect(reservation.batch.sources[0]?.metadata).not.toHaveProperty('file')
    const ready = await pipeline.confirm(reservation.batch.id, reservation.batch.sources[0]!.id)
    expect(ready.state).toBe('ready')
    await pipeline.discard(ready.id)
    const json = JSON.stringify(sourceRequests, (_key, v) =>
      typeof v === 'bigint' ? v.toString() : v,
    )
    expect(json).not.toMatch(/blob:|previewURL|https:|"file"/)
    expect(sourceRequests).toHaveLength(3)
  })
})
