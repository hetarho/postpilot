import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    title: 'AI 모델',
    active: '활성 모델',
    vanished: '등록된 모델 목록에서 사라졌어요',
    unsuitable: '이 단계에서는 쓸 수 없는 모델이에요',
  },
  en: {
    title: 'AI models',
    active: 'Active model',
    vanished: 'No longer appears in the registered model list',
    unsuitable: 'Cannot be used for this stage',
  },
} as const satisfies I18nFragment
