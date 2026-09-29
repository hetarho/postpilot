import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { ModelInfoSchema, SelectionSchema, Stage } from '@/shared/api'
import { initializeI18n } from '@/app/providers/i18n'
import { modelChoiceIssue } from '../model/access'
import { toCatalogModel, toStageSelection } from './catalog-mappers'

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
