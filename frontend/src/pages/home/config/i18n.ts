import type { I18nFragment } from '@/shared/lib'

/** Guide: 홈 — the product's promise as the screen's message, over the owner's writing kit. */
export const i18n = {
  namespace: 'creation',
  ko: {
    home: {
      eyebrow: '글쓰기 준비',
      line1: '고품질의 글을,',
      line2: '내 언어로.',
      nextAria: '다음 할 일',
    },
  },
  en: {
    home: {
      eyebrow: 'Ready to write',
      line1: 'High-quality writing,',
      line2: 'in your own words.',
      nextAria: 'What to do next',
    },
  },
} as const satisfies I18nFragment
