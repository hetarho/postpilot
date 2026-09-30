import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    select: '모델을 선택하세요',
    candidateA: '후보 A',
    candidateB: '후보 B',
    differentModels: '서로 다른 모델을 선택해 주세요.',
    pairIncomplete: '두 후보를 모두 골라야 저장돼요.',
    pair: {
      activeSaveFailed: '활성 모델을 저장하지 못했어요. 다시 골라 주세요.',
      saveFailed: 'A/B 조합을 저장하지 못했어요. 다시 시도해 주세요.',
      saving: '저장하는 중…',
      context: '컨텍스트 {{tokens}}',
    },
  },
  en: {
    select: 'Select a model',
    candidateA: 'Candidate A',
    candidateB: 'Candidate B',
    differentModels: 'Select two different models.',
    pairIncomplete: 'Choose both candidates to save the pair.',
    pair: {
      activeSaveFailed: 'Could not save the active model. Choose again.',
      saveFailed: 'Could not save the A/B pair. Try again.',
      saving: 'Saving…',
      context: '{{tokens}} context',
    },
  },
} as const satisfies I18nFragment
