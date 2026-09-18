import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { ClipProjectSchema, ClipRenderKind } from '@/shared/api'
import { toClipProject } from './clip-project'

it('keeps no render distinct from a legacy server render', () => {
  expect(
    toClipProject(create(ClipProjectSchema, { ratio: 'square' })).lastRenderKind,
  ).toBeUndefined()
  const legacy = toClipProject(
    create(ClipProjectSchema, { ratio: 'square', result: { id: 'old' } }),
  )
  expect(legacy.lastRenderKind).toBe('server')
  expect(legacy.result?.renderKind).toBe('server')
})

it('keeps the last successful kind when the plan has since changed', () => {
  const project = toClipProject(
    create(ClipProjectSchema, {
      ratio: 'square',
      editPlanRevision: 3,
      renderedPlanRevision: 2,
      lastRenderKind: ClipRenderKind.BROWSER,
      result: { id: 'browser', renderKind: ClipRenderKind.BROWSER },
    }),
  )
  expect(project.lastRenderKind).toBe('browser')
  expect(project.result?.renderKind).toBe('browser')
})
