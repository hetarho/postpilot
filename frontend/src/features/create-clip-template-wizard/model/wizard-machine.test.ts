import { createActor } from 'xstate'
import { expect, it } from 'vitest'
import {
  checkedStages,
  clipWizardMachine,
  clipWizardStateOf,
  STAGE_ROLES,
  type StageDraft,
} from './wizard-machine'
const seeds: StageDraft[] = STAGE_ROLES.map((role) => ({
  id: role,
  role,
  name: role,
  heading: role,
  intent: `${role} 내용`,
  checked: true,
  asks: role === 'info',
  fieldLabel: '가격',
}))
it('keeps edited stage order and information across Back and the same leaf, with bounded selection', () => {
  const actor = createActor(clipWizardMachine).start()
  actor.send({ type: 'choose', key: 'cafe' })
  actor.send({ type: 'choose', key: 'dessertBakery', seeds })
  actor.send({ type: 'add', name: '나만의 단계', intent: '내 내용' })
  actor.send({ type: 'add', name: '한도 밖 단계', intent: '내 내용' })
  expect(checkedStages(actor.getSnapshot().context)).toHaveLength(8)
  actor.send({ type: 'next' })
  actor.send({ type: 'reorder', from: 0, to: 7 })
  actor.send({ type: 'next' })
  actor.send({
    type: 'edit',
    id: 'info',
    patch: { heading: '알아 둘 가격', fieldLabel: '내 가격' },
  })
  actor.send({ type: 'back' })
  actor.send({ type: 'back' })
  actor.send({ type: 'back' })
  actor.send({ type: 'choose', key: 'dessertBakery', seeds })
  expect(actor.getSnapshot().context.stages.at(-1)?.id).toBe('hook')
  expect(actor.getSnapshot().context.stages.find((entry) => entry.id === 'info')).toMatchObject({
    heading: '알아 둘 가격',
    fieldLabel: '내 가격',
  })
  for (const entry of actor.getSnapshot().context.stages)
    actor.send({ type: 'toggle', id: entry.id, checked: false })
  actor.send({ type: 'next' })
  expect(clipWizardStateOf(actor.getSnapshot())).toMatchObject({ step: 'stages', attempted: true })
  actor.stop()
})
it('refuses an invalid stage without losing its answers or advancing to design', () => {
  const actor = createActor(clipWizardMachine).start()
  actor.send({ type: 'choose', key: 'cafe' })
  actor.send({ type: 'choose', key: 'dessertBakery', seeds: [seeds[0]!] })
  actor.send({ type: 'next' })
  actor.send({ type: 'next' })
  actor.send({ type: 'next' })
  actor.send({ type: 'edit', id: 'hook', patch: { intent: '두 줄\n내용' } })
  actor.send({ type: 'next' })
  expect(clipWizardStateOf(actor.getSnapshot())).toMatchObject({ step: 'stage', attempted: true })
  actor.send({ type: 'edit', id: 'hook', patch: { intent: '한 줄 내용' } })
  actor.send({ type: 'next' })
  expect(clipWizardStateOf(actor.getSnapshot()).step).toBe('design')
  actor.stop()
})
