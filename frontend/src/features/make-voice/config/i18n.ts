import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    make: {
      first: '말투 만들기',
      again: '다시 분석',
      notReady: '말투 학습에 필요한 정보가 100%가 되면 누를 수 있어요.',
      noModel: '말투 분석에 쓸 AI 모델을 먼저 골라 주세요.',
      chooseModel: 'AI 모델 고르기',
      deleted: '삭제된 말투는 분석할 수 없어요. 먼저 복원해 주세요.',
    },
  },
  en: {
    make: {
      first: 'Make the voice',
      again: 'Analyze again',
      notReady: 'Available once what the voice needs reaches 100%.',
      noModel: 'Choose an AI model for voice analysis first.',
      chooseModel: 'Choose an AI model',
      deleted: 'A deleted voice cannot be analyzed. Restore it first.',
    },
  },
} as const satisfies I18nFragment
