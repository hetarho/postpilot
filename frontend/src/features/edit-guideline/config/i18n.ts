import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16). */
export const i18n = {
  namespace: 'guidelines',
  ko: {
    edit: {
      text: '지침',
    },
  },
  en: {
    edit: {
      text: 'Guideline',
    },
  },
} as const satisfies I18nFragment
