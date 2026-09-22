import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { ExperimentOrigin, ModelExperimentSchema } from '@/shared/api'
import { toExperiment } from './experiment-mappers'

describe('a comparison origin crossing the wire', () => {
  it('reads each declared origin as the verdict form it names', () => {
    const cases: Array<[ExperimentOrigin, 'editor' | 'lab']> = [
      [ExperimentOrigin.LAB, 'lab'],
      [ExperimentOrigin.EDITOR, 'editor'],
    ]
    for (const [wire, expected] of cases) {
      const mapped = toExperiment(create(ModelExperimentSchema, { id: 'experiment', origin: wire }))
      expect(mapped.origin).toBe(expected)
    }
  })

  it('reads an unstated origin as the editor, the behaviour it predates', () => {
    const mapped = toExperiment(create(ModelExperimentSchema, { id: 'experiment' }))
    expect(mapped.origin).toBe('editor')
  })
})
