import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import {
  ComparisonPairSchema,
  ModelInfoSchema,
  PostCreditsBasis,
  SelectionSchema,
  SelectionSlot,
  Stage,
} from '@/shared/api'
import { initializeI18n } from '@/app/providers/i18n'
import { modelChoiceIssue } from '../model/access'
import { toCatalogModel, toComparisonPair, toStageSelection } from './catalog-mappers'

it('keeps raw video capability separate from each workflow transport', () => {
  const model = toCatalogModel(
    create(ModelInfoSchema, {
      videoInput: true,
      signedVideoUrl: false,
      inlineStaticVideo: true,
    }),
  )
  expect(model).toMatchObject({ videoInput: true, signedVideoUrl: false, inlineStaticVideo: true })
  expect(toCatalogModel(create(ModelInfoSchema, { videoInput: true }))).toMatchObject({
    videoInput: true,
    signedVideoUrl: false,
    inlineStaticVideo: false,
  })
})

it('maps stage access and retained locked selections without guessing an unknown grade', () => {
  const model = toCatalogModel(
    create(ModelInfoSchema, {
      stages: [Stage.WRITE],
      access: [
        {
          stage: Stage.WRITE,
          grade: 'free',
          requiredPlan: 'light',
          entitled: true,
          freePathAvailable: true,
        },
        { stage: Stage.OBSERVE, grade: 'future', requiredPlan: 'max', entitled: false },
      ],
    }),
  )
  expect(model.access?.write).toMatchObject({
    grade: 'free',
    requiredPlan: 'light',
    freePathAvailable: true,
  })
  expect(model.access?.observe).toBeUndefined()

  const saved = toStageSelection(
    create(SelectionSchema, {
      stage: Stage.WRITE,
      ref: { providerId: 'openrouter', modelId: 'paid' },
      requiredPlan: 'plus',
      unavailableReason: 'MODEL_PLAN_REQUIRED',
    }),
  )
  expect(saved).toMatchObject({
    missing: false,
    requiredPlan: 'plus',
    unavailableReason: 'MODEL_PLAN_REQUIRED',
  })
})

it('maps every generated selection slot, including optional lab candidates', () => {
  const expected: Record<SelectionSlot, string> = {
    [SelectionSlot.UNSPECIFIED]: 'active',
    [SelectionSlot.ACTIVE]: 'active',
    [SelectionSlot.CANDIDATE_A]: 'candidateA',
    [SelectionSlot.CANDIDATE_B]: 'candidateB',
    [SelectionSlot.CANDIDATE_C]: 'candidateC',
    [SelectionSlot.CANDIDATE_D]: 'candidateD',
    [SelectionSlot.CANDIDATE_E]: 'candidateE',
  }
  for (const slot of Object.values(SelectionSlot).filter(
    (value): value is SelectionSlot => typeof value === 'number',
  )) {
    expect(toStageSelection(create(SelectionSchema, { stage: Stage.WRITE, slot }))?.slot).toBe(
      expected[slot],
    )
  }
})

it('keeps optional candidates in saved C/D/E order and leaves the A/B pair separate', () => {
  const pair = toComparisonPair(
    create(ComparisonPairSchema, {
      stage: Stage.WRITE,
      candidateA: {
        stage: Stage.WRITE,
        slot: SelectionSlot.CANDIDATE_A,
        ref: { providerId: 'p', modelId: 'a' },
      },
      candidateB: {
        stage: Stage.WRITE,
        slot: SelectionSlot.CANDIDATE_B,
        ref: { providerId: 'p', modelId: 'b' },
      },
      extraCandidates: [
        {
          stage: Stage.WRITE,
          slot: SelectionSlot.CANDIDATE_C,
          ref: { providerId: 'p', modelId: 'c' },
        },
        {
          stage: Stage.WRITE,
          slot: SelectionSlot.CANDIDATE_D,
          ref: { providerId: 'p', modelId: 'd' },
        },
        {
          stage: Stage.WRITE,
          slot: SelectionSlot.CANDIDATE_E,
          ref: { providerId: 'p', modelId: 'e' },
        },
      ],
    }),
  )
  expect(pair?.candidateA?.ref.modelId).toBe('a')
  expect(pair?.candidateB?.ref.modelId).toBe('b')
  expect(pair?.extraCandidates.map((candidate) => candidate.ref.modelId)).toEqual(['c', 'd', 'e'])
})

it.each([
  ['value', 'light', 'Light'],
  ['balanced', 'basic', 'Basic'],
  ['premium', 'pro', 'Pro'],
  ['top', 'max', 'Max'],
])(
  'shows the %s grade ceiling and recovers when its %s plan is entitled',
  (grade, requiredPlan, label) => {
    initializeI18n('ko')
    const info = create(ModelInfoSchema, {
      stages: [Stage.WRITE],
      affordable: true,
      access: [
        {
          stage: Stage.WRITE,
          grade,
          requiredPlan,
          entitled: false,
          unavailableReason: 'MODEL_PLAN_REQUIRED',
        },
      ],
    })
    expect(modelChoiceIssue(toCatalogModel(info), 'write')).toBe(`${label} 요금제부터 쓸 수 있어요`)
    info.access[0]!.entitled = true
    info.access[0]!.unavailableReason = ''
    expect(modelChoiceIssue(toCatalogModel(info), 'write')).toBe('')
  },
)

// QUOTA-64: a stage's per-post figure crosses as credits and a basis; a stage or basis this
// build cannot name is dropped rather than shown unlabelled.
it('maps per-post credits by stage and drops what it cannot name', () => {
  const model = toCatalogModel(
    create(ModelInfoSchema, {
      postCredits: [
        { stage: Stage.OBSERVE, credits: 30, basis: PostCreditsBasis.ESTIMATE },
        { stage: Stage.WRITE, credits: 12, basis: PostCreditsBasis.RECENT_USAGE },
        { stage: Stage.ANALYZE, credits: 9, basis: PostCreditsBasis.UNSPECIFIED },
      ],
    }),
  )
  expect(model.postCredits).toEqual({
    observe: { credits: 30, basis: 'estimate' },
    write: { credits: 12, basis: 'recent' },
  })
  expect(toCatalogModel(create(ModelInfoSchema, {})).postCredits).toBeUndefined()
})
