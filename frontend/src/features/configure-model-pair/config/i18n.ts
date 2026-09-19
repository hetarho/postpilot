import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    select: '모델을 선택하세요',
    candidateA: '후보 A',
    candidateB: '후보 B',
    savePair: 'A/B 조합 저장',
    differentModels: '서로 다른 모델을 선택해 주세요.',
    pair: {
      activeSaveFailed: '활성 모델을 저장하지 못했어요. 다시 골라 주세요.',
      saveFailed: 'A/B 조합을 저장하지 못했어요. 다시 시도해 주세요.',
      saved: 'A/B 조합을 저장했어요.',
      saving: '저장하는 중…',
      pricing: '컨텍스트 {{tokens}} · 1M 토큰 기준 입력 ${{input}} / 출력 ${{output}}',
      priceUnchecked: '가격 미확인',
    },
  },
  en: {
    select: 'Select a model',
    candidateA: 'Candidate A',
    candidateB: 'Candidate B',
    savePair: 'Save A/B pair',
    differentModels: 'Select two different models.',
    pair: {
      activeSaveFailed: 'Could not save the active model. Choose again.',
      saveFailed: 'Could not save the A/B pair. Try again.',
      saved: 'Saved the A/B pair.',
      saving: 'Saving…',
      pricing: '{{tokens}} context · per 1M tokens: ${{input}} input / ${{output}} output',
      priceUnchecked: 'Price not checked',
    },
  },
} as const satisfies I18nFragment
