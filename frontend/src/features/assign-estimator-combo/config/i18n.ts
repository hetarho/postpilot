import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    combos: {
      title: '편수 기준 조합',
      description:
        '플랜 화면이 "매달 약 몇 편"을 계산할 때 쓰는 모델입니다. 두 모델을 모두 고르면 그 등급의 실제 단가로 편수를 계산합니다.',
      name: {
        quality: '품질',
        balanced: '균형',
        value: '가성비',
        cheapest: '최저가',
      },
      observe: '사진 분석',
      write: '글 작성',
      none: '고르지 않음',
      unassigned: '두 모델을 모두 고르기 전에는 플랜 화면에서 이 등급의 편수를 보여주지 않습니다.',
      retired: '{{model}} · 지금은 등록되지 않은 모델',
      loadFailed: '조합을 불러오지 못했어요.',
    },
  },
  en: {
    combos: {
      title: 'Post estimate combos',
      description:
        'The models the plan screen prices "about N posts a month" with. Choose both and the estimate uses that tier\'s real rates.',
      name: {
        quality: 'Quality',
        balanced: 'Balanced',
        value: 'Value',
        cheapest: 'Cheapest',
      },
      observe: 'Photo analysis',
      write: 'Writing',
      none: 'Not chosen',
      unassigned: 'Until both models are chosen, the plan screen shows no estimate for this tier.',
      retired: '{{model}} · no longer registered',
      loadFailed: 'Could not load the combos.',
    },
  },
} as const satisfies I18nFragment
