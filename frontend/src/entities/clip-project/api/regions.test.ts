import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { ClipProjectSchema } from '@/shared/api'
import { toClipProject } from './clip-project'

it('preserves disabled regions and explicitly cleared owner text on reads', () => {
  const project = toClipProject(
    create(ClipProjectSchema, {
      ratio: 'vertical',
      regions: {
        revision: 4,
        intro: {
          enabled: false,
          slots: [
            {
              id: 'project-intro-1',
              text: '',
              ownerFixed: true,
              instruction: 'Keep this',
              instructionEdited: true,
            },
          ],
        },
        outro: { enabled: true, slots: [{ id: 'project-outro-1', text: 'Bound', bound: true }] },
      },
    }),
  )
  expect(project.regions?.revision).toBe(4)
  expect(project.regions?.intro.enabled).toBe(false)
  expect(project.regions?.intro.slots[0]).toEqual({
    id: 'project-intro-1',
    text: '',
    ownerFixed: true,
    instruction: 'Keep this',
    instructionEdited: true,
    bound: false,
  })
  expect(project.regions?.outro.slots[0].bound).toBe(true)
})

it('does not infer legacy enablement from stored preset names', () => {
  expect(
    toClipProject(
      create(ClipProjectSchema, {
        ratio: 'vertical',
        introPreset: 'serif',
        outroPreset: 'credits',
      }),
    ).regions,
  ).toBeUndefined()
})
