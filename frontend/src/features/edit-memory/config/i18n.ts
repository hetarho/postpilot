import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `memories` namespace (ARCH-16). */
export const i18n = {
  namespace: 'memories',
  ko: {
    edit: {
      text: '기억',
    },
  },
  en: {
    edit: {
      text: 'Memory',
    },
  },
} as const satisfies I18nFragment
