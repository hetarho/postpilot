import { expect, it } from 'vitest'
import type { ClipProject } from '@/entities/clip-project'
import { confirmationRefusal } from './confirmation-refusal'

const project = (overrides: Partial<ClipProject> = {}): ClipProject =>
  ({
    id: 'clip',
    editPlanRevision: 2,
    renderedPlanRevision: 0,
    canConfirm: true,
    ...overrides,
  }) as ClipProject

it('is ready when the saved plan can be confirmed', () => {
  expect(confirmationRefusal(project())).toBeUndefined()
})

it('holds a finalized project and one a job is running on', () => {
  expect(
    confirmationRefusal(project({ finalized: { at: 'x', planRevision: 2, resultId: 'r' } })),
  ).toBe('finalized')
  expect(
    confirmationRefusal(
      project({
        latestJob: { id: 'j', kind: 'revise_clip', status: 'running' } as ClipProject['latestJob'],
      }),
    ),
  ).toBe('busy')
})

it('states the server’s verdict on the saved plan, narration included', () => {
  expect(confirmationRefusal(project({ confirmationRefusal: 'narration' }))).toBe('narration')
  expect(confirmationRefusal(project({ confirmationRefusal: 'invalid_plan' }))).toBe('invalid_plan')
  expect(confirmationRefusal(project({ canConfirm: false }))).toBe('unavailable')
})

// An unsaved draft is saved by the press, so a verdict on the older plan does not hold it.
it('leaves the plan’s own verdict to the press while edits are unsaved', () => {
  expect(
    confirmationRefusal(project({ confirmationRefusal: 'narration' }), { unsaved: true }),
  ).toBeUndefined()
  expect(confirmationRefusal(project({ confirmationRefusal: 'busy' }), { unsaved: true })).toBe(
    'busy',
  )
})
