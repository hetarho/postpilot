import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { ClipProjectSchema } from '@/shared/api'
import { clipObservationsFixture } from '@/test/clip-observations'
import { toClipProject } from './clip-project'

it('maps terminal evidence separately and refuses invalid range playback and counts', () => {
  const proto = create(ClipProjectSchema, {
    ratio: 'vertical',
    attemptInspection: {
      jobId: 'attempt',
      status: 'available',
      totalSources: 2,
      completedSources: 1,
      totalChunks: 3,
      completedChunks: 2,
      observations: clipObservationsFixture(),
      ranges: [
        { cut: 1, source: 1, startMs: 3500, endMs: 10500, valid: true },
        { cut: 2, source: 99, startMs: 0, endMs: 90000, valid: true },
      ],
      measurements: {
        input_bytes: 37198,
        input_limit_bytes: 27952,
        content_bytes: -1,
        schema_bytes: 180000001,
        target_ms: 30000,
        after_ms: -1,
        raw_start_ms: -200,
        focal_x_ppm: -100000,
        subject_width_ppm: 180000001,
        private_field: 10,
      },
    },
  })
  const p = toClipProject(proto)
  expect(p.observations).toBeUndefined()
  expect(p.editing).toBeUndefined()
  expect(p.result).toBeUndefined()
  expect(p.attemptInspection?.ranges.map((r) => r.valid)).toEqual([true, false])
  expect(p.attemptInspection?.measurements).toEqual({
    input_bytes: 37198,
    input_limit_bytes: 27952,
    target_ms: 30000,
    raw_start_ms: -200,
    focal_x_ppm: -100000,
  })
  proto.attemptInspection!.completedChunks = 4
  expect(toClipProject(proto).attemptInspection?.status).toBe('unavailable')
})
