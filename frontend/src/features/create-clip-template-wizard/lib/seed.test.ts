import { createActor } from 'xstate'
import { expect, it } from 'vitest'
import { CLIP_COMPOSITION_LIMITS } from '@/entities/clip-design'
import {
  checkedStages,
  clipWizardMachine,
  STAGE_ROLES,
  type StageDraft,
} from '../model/wizard-machine'
import { clipWizardSeed, firstSentence, type ClipWizardSeed } from './seed'

const roles: StageDraft[] = STAGE_ROLES.map((role) => ({
  id: role,
  role,
  name: role,
  heading: role,
  intent: `${role} 내용`,
  checked: true,
  asks: false,
  fieldLabel: role,
}))

it('reads a paragraph’s first sentence on one line, bounded by the stage line limit', () => {
  expect(firstSentence('  입구에서 간판을 보여 줘요. 그다음 매장 안으로 들어가요.')).toBe(
    '입구에서 간판을 보여 줘요.',
  )
  expect(firstSentence('첫 줄은 마침표가 없고\n둘째 줄')).toBe('첫 줄은 마침표가 없고')
  expect(firstSentence('소수점 3.5배도 문장 안이에요! 끝')).toBe('소수점 3.5배도 문장 안이에요!')
  expect(Array.from(firstSentence('가'.repeat(500)))).toHaveLength(
    CLIP_COMPOSITION_LIMITS.promptChars,
  )
})

it('makes one stage per paragraph with text and keeps only the presets the clip chose', () => {
  expect(
    clipWizardSeed({
      paragraphs: [{ text: '크림 라떼를 가까이 보여 줘요. 거품이 가득해요.' }, { text: '  ' }],
      design: { introPreset: '', outroPreset: 'credits', allowedCaptionStyles: ['bold'] },
    }),
  ).toEqual({
    stages: [{ line: '크림 라떼를 가까이 보여 줘요.' }],
    design: { outroPreset: 'credits', allowedCaptionStyles: ['bold'] },
  })
})

it('starts the wizard with the design preselected and the clip’s stages checked and prefilled, unnamed', () => {
  const seed: ClipWizardSeed = {
    stages: [{ line: '입구를 보여 줘요.' }, { line: '대표 메뉴를 보여 줘요.' }],
    design: { introPreset: 'b', outroPreset: 'credits', allowedCaptionStyles: ['bold', 'neon'] },
  }
  const actor = createActor(clipWizardMachine, { input: { seed } }).start()
  expect(actor.getSnapshot().context).toMatchObject({
    introPreset: 'b',
    outroPreset: 'credits',
    allowedCaptionStyles: ['bold', 'neon'],
  })
  actor.send({ type: 'choose', key: 'cafe' })
  actor.send({ type: 'choose', key: 'dessertBakery', seeds: roles })
  const context = actor.getSnapshot().context
  expect(checkedStages(context).map((stage) => [stage.role, stage.intent, stage.heading])).toEqual([
    ['custom', '입구를 보여 줘요.', ''],
    ['custom', '대표 메뉴를 보여 줘요.', ''],
  ])
  // The leaf's roles stay on offer, unchecked, beside the clip's own form.
  expect(context.stages.filter((stage) => stage.role !== 'custom')).toHaveLength(roles.length)
  // Each stage is named in its own step: an unnamed one holds the headings step.
  actor.send({ type: 'next' })
  actor.send({ type: 'next' })
  actor.send({ type: 'next' })
  expect(actor.getSnapshot().value).toBe('headings')
  expect(context.customCount).toBe(2)
  actor.stop()
})

it('leaves the ordinary wizard unchanged without a seed', () => {
  const actor = createActor(clipWizardMachine, { input: {} }).start()
  actor.send({ type: 'choose', key: 'cafe' })
  actor.send({ type: 'choose', key: 'dessertBakery', seeds: roles })
  expect(checkedStages(actor.getSnapshot().context)).toHaveLength(roles.length)
  actor.stop()
})
