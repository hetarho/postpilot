import { describe, expect, it } from 'vitest'
import { clipState } from './state'

const result = { contentType: 'video/mp4', bytes: 1, durationMs: 1000, createdAt: '2026-09-11' }

describe('clipState', () => {
  it('is a draft while no plan has been written', () => {
    expect(clipState({ editPlanRevision: 0, renderedPlanRevision: 0, result: undefined })).toBe(
      'draft',
    )
  })

  it('never hides a rendered result behind the draft step', () => {
    expect(clipState({ editPlanRevision: 0, renderedPlanRevision: 0, result })).toBe('finished')
  })

  it('is finished when the render matches the plan', () => {
    expect(clipState({ editPlanRevision: 3, renderedPlanRevision: 3, result })).toBe('finished')
  })

  it('is refining when the plan is newer than its render', () => {
    expect(clipState({ editPlanRevision: 4, renderedPlanRevision: 3, result })).toBe('refining')
  })

  it('is refining when a plan exists but nothing has rendered', () => {
    expect(clipState({ editPlanRevision: 1, renderedPlanRevision: 0, result: undefined })).toBe(
      'refining',
    )
  })
})
