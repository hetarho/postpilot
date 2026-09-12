import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { ClipProjectSchema } from '@/shared/api'
import { toClipProject } from './clip-project'

describe('clip confirmation identity', () => {
  it('keeps an old successful render unfinalized and allows its download', () => {
    const project = toClipProject(
      create(ClipProjectSchema, {
        ratio: 'square',
        editPlanRevision: 2,
        renderedPlanRevision: 2,
        result: { id: 'legacy-result', downloadUrl: 'https://example.test/result' },
      }),
    )
    expect(project.finalized).toBeUndefined()
    expect(project.result?.id).toBe('legacy-result')
    expect(project.result?.downloadUrl).toBe('https://example.test/result')
  })
  it('keeps server confirmation and refusal projections distinct from render success', () => {
    const project = toClipProject(
      create(ClipProjectSchema, {
        ratio: 'square',
        editPlanRevision: 2,
        renderedPlanRevision: 2,
        finalizedAt: '2026-09-13T00:00:00Z',
        finalizedPlanRevision: 2,
        finalizedResultId: 'result-2',
        result: { id: 'result-2' },
        canEdit: false,
        canFinalize: false,
        finalizationRefusal: 'finalized',
      }),
    )
    expect(project.finalized).toEqual({
      at: '2026-09-13T00:00:00Z',
      planRevision: 2,
      resultId: 'result-2',
    })
    expect(project.canEdit).toBe(false)
    expect(project.canFinalize).toBe(false)
    expect(project.finalizationRefusal).toBe('finalized')
  })
  it.each([
    { finalizedAt: 'invalid' },
    { finalizedAt: '2026-09-13T00:00:00Z' },
    { finalizedResultId: 'result-2', finalizedPlanRevision: 2 },
    {
      finalizedAt: '2026-09-13T00:00:00Z',
      finalizedResultId: 'result-2',
      finalizedPlanRevision: 2,
      result: { id: 'different' },
    },
  ])('refuses a partial or inconsistent confirmation', (fields) => {
    expect(() => toClipProject(create(ClipProjectSchema, { ratio: 'square', ...fields }))).toThrow(
      'Invalid clip finalization',
    )
  })
})
