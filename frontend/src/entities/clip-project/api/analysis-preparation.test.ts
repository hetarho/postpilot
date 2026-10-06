import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { ClipAnalysisPreparationResponseSchema } from '@/shared/api'
import { toClipAnalysisPreparation } from './analysis-preparation'

function response() {
  return create(ClipAnalysisPreparationResponseSchema, {
    preparationId: 'preparation',
    projectId: 'project',
    batchId: 'batch',
    expectedRevision: 3,
    state: 'preparing',
    expiresAt: '2026-10-07T12:00:00Z',
    originalMeasurementProvenance: 'browser_client',
    profile: {
      version: 'clip-browser-analysis-v1',
      intervalMs: 60_000,
      longEdge: 720,
      framesPerSecond: 15,
      maxCopyBytes: 8_388_608n,
      videoCodec: 'h264',
      pixelFormat: 'yuv420p',
      audioCodec: 'aac',
      audioRate: 48_000,
      audioChannels: 1,
      audioBitrate: 64_000,
      qualified: false,
    },
    copies: [
      {
        slot: 'analysis/source/0',
        sourceId: 'source',
        fingerprint: 'a'.repeat(64),
        ordinal: 0,
        offsetMs: 0,
        durationMs: 60_000,
        width: 720,
        height: 404,
        state: 'expected',
      },
    ],
  })
}
describe('browser analysis authorization metadata', () => {
  it('preserves explicit client provenance, finite slots and the independent quality gate', () => {
    const result = toClipAnalysisPreparation(response())
    expect(result.profile.qualified).toBe(false)
    expect(result.originalMeasurementProvenance).toBe('browser_client')
    expect(result.copies[0]).toMatchObject({
      slot: 'analysis/source/0',
      offsetMs: 0,
      durationMs: 60_000,
      bytes: 0,
    })
    expect(result.copies[0]).not.toHaveProperty('$typeName')
  })
  it.each(['profile', 'provenance', 'duration', 'bytes', 'coverage', 'state'])(
    'rejects forged %s',
    (field) => {
      const value = response()
      switch (field) {
        case 'profile':
          value.profile!.version = 'other-profile'
          break
        case 'provenance':
          value.originalMeasurementProvenance = 'native_original'
          break
        case 'duration':
          value.copies[0]!.durationMs = 65_000
          break
        case 'bytes':
          value.copies[0]!.bytes = 8_388_609n
          break
        case 'coverage':
          value.copies[0]!.offsetMs = 1
          break
        case 'state':
          value.state = 'approved'
          break
      }
      expect(() => toClipAnalysisPreparation(value)).toThrow()
    },
  )
})
