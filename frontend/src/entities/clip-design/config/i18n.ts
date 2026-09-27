import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    ratio: { vertical: '세로 9:16', horizontal: '가로 16:9', square: '정방형 1:1' },
    accent: {
      none: '기본',
      coral: '코랄',
      amber: '앰버',
      lime: '라임',
      teal: '청록',
      blue: '파랑',
      violet: '보라',
      pink: '분홍',
    },
  },
  en: {
    ratio: { vertical: 'Vertical 9:16', horizontal: 'Horizontal 16:9', square: 'Square 1:1' },
    accent: {
      none: 'Neutral',
      coral: 'Coral',
      amber: 'Amber',
      lime: 'Lime',
      teal: 'Teal',
      blue: 'Blue',
      violet: 'Violet',
      pink: 'Pink',
    },
  },
} as const satisfies I18nFragment
