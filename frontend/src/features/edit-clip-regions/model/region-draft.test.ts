import { expect, it } from 'vitest'
import type { ClipProjectRegions } from '@/entities/clip-project'
import {
  noRegionEdits,
  regionPatch,
  sendableRegionEdits,
  slotState,
  unsavedRegionEdits,
  withRegionEdits,
  withSlotEdit,
} from './region-draft'

const slot = (id: string, text = '', extra = {}) => ({
  id,
  instruction: '',
  text,
  instructionEdited: false,
  ownerFixed: false,
  bound: false,
  ...extra,
})
const server = (): ClipProjectRegions => ({
  revision: 3,
  intro: {
    enabled: true,
    slots: [slot('project-intro-1', '생성된 첫 줄'), slot('project-intro-2')],
  },
  outro: { enabled: false, slots: [slot('project-outro-1', '다시 만나요', { ownerFixed: true })] },
})

// CLIP-186: typing a slot's final text makes it the owner's from that keystroke, an empty one
// included; an instruction alone changes neither the words nor who owns them.
it('shows the owner’s edits over the server’s slots', () => {
  let edits = withSlotEdit(noRegionEdits(), 'intro', 'project-intro-1', { text: '' })
  edits = withSlotEdit(edits, 'intro', 'project-intro-2', { instruction: '가게 이름' })
  const shown = withRegionEdits(server(), edits)
  expect(shown.intro.slots[0]).toMatchObject({ text: '', ownerFixed: true })
  expect(slotState(shown.intro.slots[0])).toBe('blank')
  expect(shown.intro.slots[1]).toMatchObject({
    instruction: '가게 이름',
    instructionEdited: true,
    ownerFixed: false,
  })
  expect(slotState(shown.intro.slots[1])).toBe('awaiting')
  expect(slotState(server().intro.slots[0])).toBe('written')
  expect(slotState(slot('x', '대표 메뉴', { bound: true }))).toBe('bound')
})

// T450: the patch names only what changed, and false and '' are real values.
it('patches only the fields the owner changed', () => {
  const edits = withSlotEdit(noRegionEdits(), 'intro', 'project-intro-1', { text: '' })
  expect(regionPatch(edits.intro)).toEqual({ slots: [{ id: 'project-intro-1', text: '' }] })
  expect(regionPatch({ enabled: false, slots: {} })).toEqual({ enabled: false, slots: [] })
  expect(regionPatch(edits.outro)).toBeUndefined()
})

// A saved field leaves the owner's edits, so a later change made elsewhere is not overwritten
// by it; what was typed during the flight stays.
it('drops what the server took and keeps what it has not', () => {
  let edits = withSlotEdit(noRegionEdits(), 'intro', 'project-intro-1', { text: '첫 줄' })
  edits = withSlotEdit(edits, 'intro', 'project-intro-2', { text: '둘째 줄' })
  const saved = server()
  saved.intro.slots[0] = { ...saved.intro.slots[0], text: '첫 줄', ownerFixed: true }
  const left = unsavedRegionEdits(saved, edits)
  expect(left.intro.slots).toEqual({ 'project-intro-2': { text: '둘째 줄' } })
  // The same words as generated are still the owner's to fix.
  const same = withSlotEdit(noRegionEdits(), 'intro', 'project-intro-1', { text: '생성된 첫 줄' })
  expect(unsavedRegionEdits(server(), same).intro.slots).toEqual({
    'project-intro-1': { text: '생성된 첫 줄' },
  })
})

// CLIP-189: an active slot's text its slot cannot draw is held in the field, not sent; words
// past the preset's slots and a region that is off are not checked, since nothing draws them.
it('holds back an active slot’s text its slot cannot draw', () => {
  let edits = withSlotEdit(noRegionEdits(), 'intro', 'project-intro-1', {
    text: '너무 긴 문구',
    instruction: '가게 이름',
  })
  edits = withSlotEdit(edits, 'intro', 'project-intro-2', { text: '너무 긴 둘째 줄' })
  edits = withSlotEdit(edits, 'outro', 'project-outro-1', { text: '너무 긴 인사' })
  const sendable = sendableRegionEdits(
    server(),
    edits,
    () => 1,
    () => false,
  )
  expect(sendable.intro.slots).toEqual({
    'project-intro-1': { instruction: '가게 이름' },
    'project-intro-2': { text: '너무 긴 둘째 줄' },
  })
  expect(sendable.outro.slots).toEqual({ 'project-outro-1': { text: '너무 긴 인사' } })
})
