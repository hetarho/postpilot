import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { ClipObservationsSchema, ClipProjectSchema } from '@/shared/api'
import { clipObservationsFixture } from '@/test/clip-observations'
import { toClipObservations } from './observations'
import { toClipProject } from './clip-project'

describe('clip observation mapping', () => {
  it('preserves recorded fields through the project detail contract', () => {
    const observations = clipObservationsFixture()
    const project = create(ClipProjectSchema, { ratio: 'vertical', observations })
    expect(toClipProject(project).observations).toEqual(observations)
    expect(
      toClipProject(create(ClipProjectSchema, { ratio: 'vertical' })).observations,
    ).toBeUndefined()
  })
  it('treats unknown status or missing source identity as unavailable', () => {
    for (const data of [
      { status: 'future' },
      { status: 'available', sources: [{}] },
      { status: 'available', sources: [{ source: { id: 'source' } }] },
    ]) {
      expect(toClipObservations(create(ClipObservationsSchema, data))).toEqual({
        status: 'unavailable',
        sources: [],
      })
    }
    expect(toClipObservations(create(ClipObservationsSchema, { status: 'empty' }))).toEqual({
      status: 'empty',
      sources: [],
    })
  })
})
