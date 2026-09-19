import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    recommendation: {
      unaffordable: '{{models}} 모델을 쓰려면 크레딧이 부족해요.',
      description: '활성 모델과 세 단계 A/B 쌍 9개를 한 번에 저장합니다.',
      apply: '추천 조합 적용',
      applied: '세 단계의 활성 모델과 A/B 조합을 적용했어요.',
      failed: '추천 조합을 적용하지 못했어요.',
    },
  },
  en: {
    recommendation: {
      unaffordable: 'Your credits do not cover {{models}}.',
      description: 'Saves the active models and nine A/B pairs across all three stages at once.',
      apply: 'Apply recommendation',
      applied: 'Applied the active models and A/B pairs for all three stages.',
      failed: 'Could not apply the recommendation.',
    },
  },
} as const satisfies I18nFragment
