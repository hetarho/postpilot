import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'voices',
  ko: {
    writingCandidates: {
      created: 'AI가 만든 말투',
      fictional: '가상의 상황으로 쓴 예시',
      feel: '이런 느낌이에요',
      choose: '이 스타일 고르기',
      selected: '선택한 스타일',
    },
  },
  en: {
    writingCandidates: {
      created: 'AI-created writing style',
      fictional: 'Example from a fictional situation',
      feel: 'How it feels',
      choose: 'Choose this style',
      selected: 'Selected style',
    },
  },
} as const satisfies I18nFragment
