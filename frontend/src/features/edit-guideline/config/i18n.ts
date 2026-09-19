import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16). */
export const i18n = {
  namespace: 'guidelines',
  ko: {
    edit: {
      text: '지침',
      scope: '적용 범위',
    },
  },
  en: {
    edit: {
      text: 'Guideline',
      scope: 'Applies to',
    },
  },
} as const satisfies I18nFragment
