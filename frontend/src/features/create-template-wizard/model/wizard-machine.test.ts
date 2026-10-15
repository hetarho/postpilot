import { describe, expect, it } from 'vitest'
import { createActor } from 'xstate'
import { TEMPLATE_ASK_LABEL_MAX_CHARS, TEMPLATE_PHOTO_ROW_MAX } from '@/entities/template'
import { ROLES, nodeAt, type RoleKey } from '../config/catalog'
import {
  checkedSections,
  initialWizardState,
  sectionProblem,
  wizardHasAnswers,
  wizardProgress,
  wizardTransition,
  wizardMachine,
  type SectionDraft,
  type WizardEvent,
  type WizardState,
} from './wizard-machine'

/** The leaf's seeds as the screen makes them, with the role key as its name. */
function seeds(path: string[]): SectionDraft[] {
  return (nodeAt(path)?.roles ?? []).map((role: RoleKey) => {
    const defaults = ROLES[role] as {
      content: SectionDraft['content']
      photos: number
      repeat?: boolean
    }
    return {
      id: role,
      role,
      name: role,
      help: `${role} help`,
      checked: true,
      heading: role,
      content: defaults.content,
      question: role,
      required: false,
      literal: '',
      photos: defaults.photos,
      repeatable: defaults.repeat !== undefined,
      repeat: defaults.repeat ?? false,
    }
  })
}

function run(...events: WizardEvent[]): WizardState {
  return events.reduce(wizardTransition, initialWizardState())
}

const toMeal: WizardEvent[] = [
  { type: 'choose', key: 'restaurant' },
  { type: 'choose', key: 'meal', seeds: seeds(['restaurant', 'meal']) },
]

describe('narrowing the target', () => {
  it('freezes post headings and appends checked custom sections only after a leaf', () => {
    const headings = ['첫인상', '다시 가고 싶은 이유']
    const actor = createActor(wizardMachine, { input: { seedHeadings: headings } }).start()
    headings[0] = 'late source edit'
    expect(actor.getSnapshot().context.sections).toEqual([])
    actor.send(toMeal[0]!)
    expect(actor.getSnapshot().context.sections).toEqual([])
    actor.send(toMeal[1]!)
    expect(actor.getSnapshot().context.sections.slice(-2)).toMatchObject([
      { id: 'custom-1', name: '첫인상', heading: '첫인상', checked: true, help: '' },
      { id: 'custom-2', name: '다시 가고 싶은 이유', checked: true },
    ])
    actor.send({ type: 'add', name: '내 추가 부분', help: '추가 설명' })
    expect(actor.getSnapshot().context.sections.at(-1)?.id).toBe('custom-3')
    actor.send({ type: 'toggle', id: 'custom-1', checked: false })
    actor.send({ type: 'back' })
    actor.send(toMeal[1]!)
    expect(
      actor.getSnapshot().context.sections.filter((section) => section.id === 'custom-1'),
    ).toHaveLength(1)
    expect(
      actor.getSnapshot().context.sections.find((section) => section.id === 'custom-1')?.checked,
    ).toBe(false)
    actor.stop()
  })
  it('asks one question at a time until a leaf, then offers its sections', () => {
    const branch = run({ type: 'choose', key: 'restaurant' })
    expect(branch).toMatchObject({ step: 'target', path: ['restaurant'], depth: 1 })
    expect(wizardHasAnswers(branch)).toBe(true)
    const leaf = run(...toMeal)
    expect(leaf.step).toBe('sections')
    expect(leaf.sections.map((section) => section.id)).toEqual(
      nodeAt(['restaurant', 'meal'])!.roles,
    )
    expect(leaf.sections.every((section) => section.checked)).toBe(true)
  })

  it('walks a product through its kind and review form', () => {
    const state = run(
      { type: 'choose', key: 'product' },
      { type: 'choose', key: 'cosmetics' },
      { type: 'choose', key: 'unboxing', seeds: seeds(['product', 'cosmetics', 'unboxing']) },
    )
    expect(state.step).toBe('sections')
    expect(state.sections[0]!.id).toBe('components')
    expect(state.sections.map((section) => section.id)).toContain('textureSpread')
    expect(state.sections.map((section) => section.id)).not.toContain('usageReview')
  })

  it('refuses a choice the tree does not hold and a leaf without seeds', () => {
    const start = initialWizardState()
    expect(wizardTransition(start, { type: 'choose', key: 'clinic' })).toBe(start)
    const branch = run({ type: 'choose', key: 'restaurant' })
    expect(wizardTransition(branch, { type: 'choose', key: 'meal' })).toBe(branch)
  })
})

describe('Back', () => {
  it('keeps every answer, and the same leaf again keeps its sections', () => {
    const edited = run(
      ...toMeal,
      { type: 'toggle', id: 'service', checked: false },
      { type: 'next' },
      { type: 'next' },
      { type: 'edit', id: 'visitReason', patch: { heading: '찾아간 이유' } },
    )
    expect(edited.step).toBe('headings')
    const back = run(
      ...toMeal,
      { type: 'toggle', id: 'service', checked: false },
      { type: 'next' },
      { type: 'next' },
      { type: 'edit', id: 'visitReason', patch: { heading: '찾아간 이유' } },
      { type: 'back' },
      { type: 'back' },
      { type: 'back' },
    )
    expect(back).toMatchObject({ step: 'target', path: ['restaurant', 'meal'], depth: 1 })
    const again = wizardTransition(back, {
      type: 'choose',
      key: 'meal',
      seeds: seeds(['restaurant', 'meal']),
    })
    expect(again.sections.find((section) => section.id === 'service')!.checked).toBe(false)
    expect(again.sections.find((section) => section.id === 'visitReason')!.heading).toBe(
      '찾아간 이유',
    )
    // Another leaf has other sections.
    const other = wizardTransition(back, {
      type: 'choose',
      key: 'bar',
      seeds: seeds(['restaurant', 'bar']),
    })
    expect(other.sections.map((section) => section.id)).toContain('drinksSnacks')
    expect(other.sections.find((section) => section.id === 'visitReason')!.heading).toBe(
      'visitReason',
    )
  })

  it('walks the section steps back to the headings and the first question', () => {
    let state = run(
      ...toMeal,
      { type: 'next' },
      { type: 'next' },
      { type: 'next' },
      { type: 'next' },
    )
    expect(state).toMatchObject({ step: 'section', cursor: 1 })
    state = wizardTransition(state, { type: 'back' })
    expect(state).toMatchObject({ step: 'section', cursor: 0 })
    state = wizardTransition(state, { type: 'back' })
    expect(state.step).toBe('headings')
  })
})

describe('the guards', () => {
  it('needs one checked section and says so only after a refused 다음', () => {
    let state = run(...toMeal)
    for (const section of state.sections)
      state = wizardTransition(state, { type: 'toggle', id: section.id, checked: false })
    expect(state.attempted).toBe(false)
    state = wizardTransition(state, { type: 'next' })
    expect(state).toMatchObject({ step: 'sections', attempted: true })
  })

  it('needs every heading, within its bound', () => {
    let state = run(...toMeal, { type: 'next' }, { type: 'next' })
    state = wizardTransition(state, { type: 'edit', id: 'exterior', patch: { heading: '  ' } })
    expect(wizardTransition(state, { type: 'next' })).toMatchObject({
      step: 'headings',
      attempted: true,
    })
    state = wizardTransition(state, {
      type: 'edit',
      id: 'exterior',
      patch: { heading: '가'.repeat(TEMPLATE_ASK_LABEL_MAX_CHARS + 1) },
    })
    expect(wizardTransition(state, { type: 'next' }).step).toBe('headings')
  })

  it('needs a question for 직접 입력, unique among the sections, and text for 고정 문구', () => {
    let state = run(...toMeal, { type: 'next' }, { type: 'next' }, { type: 'next' })
    expect(state).toMatchObject({ step: 'section', cursor: 0 })
    state = wizardTransition(state, {
      type: 'edit',
      id: 'visitReason',
      patch: { content: 'ask', question: 'storeInfo' },
    })
    expect(sectionProblem(state.sections[0]!, state.sections)).toBe('question_repeated')
    expect(wizardTransition(state, { type: 'next' })).toMatchObject({ cursor: 0, attempted: true })
    state = wizardTransition(state, {
      type: 'edit',
      id: 'visitReason',
      patch: { content: 'literal' },
    })
    expect(sectionProblem(state.sections[0]!, state.sections)).toBe('literal_missing')
    state = wizardTransition(state, {
      type: 'edit',
      id: 'visitReason',
      patch: { literal: '안녕하세요' },
    })
    expect(wizardTransition(state, { type: 'next' })).toMatchObject({ cursor: 1, attempted: false })
  })

  it('clamps photos to the row and repeats only a repeatable role', () => {
    let state = run(...toMeal, { type: 'next' }, { type: 'next' })
    // Sections are edited on the headings and section steps; the checklist only checks them.
    expect(
      wizardTransition(run(...toMeal), {
        type: 'edit',
        id: 'exterior',
        patch: { photos: 2 },
      }).sections.find((section) => section.id === 'exterior')!.photos,
    ).toBe(1)
    state = wizardTransition(state, {
      type: 'edit',
      id: 'exterior',
      patch: { photos: 99, repeat: true },
    })
    const exterior = state.sections.find((section) => section.id === 'exterior')!
    expect(exterior).toMatchObject({ photos: TEMPLATE_PHOTO_ROW_MAX, repeat: false })
    expect(state.sections.find((section) => section.id === 'orderedMenu')!.repeat).toBe(true)
  })

  it('reorders the checked sections and counts the steps once the target is known', () => {
    let state = run(...toMeal, { type: 'toggle', id: 'service', checked: false }, { type: 'next' })
    expect(wizardProgress(state)).toEqual({ current: 2, total: 3 + 10 + 1 })
    state = wizardTransition(state, { type: 'reorder', from: 0, to: 2 })
    expect(
      checkedSections(state)
        .slice(0, 3)
        .map((section) => section.id),
    ).toEqual(['locationParking', 'exterior', 'visitReason'])
    expect(state.sections.at(-1)!.id).toBe('service')
    expect(wizardProgress(initialWizardState())).toBeUndefined()
  })

  it('adds a custom section checked, and removes only a custom one', () => {
    let state = run(...toMeal, { type: 'add', name: ' 주차 꿀팁 ', help: '골목 주차 자리' })
    const custom = state.sections.at(-1)!
    expect(custom).toMatchObject({
      id: 'custom-1',
      name: '주차 꿀팁',
      heading: '주차 꿀팁',
      checked: true,
    })
    state = wizardTransition(state, { type: 'remove', id: 'verdict' })
    expect(state.sections.some((section) => section.id === 'verdict')).toBe(true)
    state = wizardTransition(state, { type: 'remove', id: 'custom-1' })
    expect(state.sections.some((section) => section.id === 'custom-1')).toBe(false)
  })
})
