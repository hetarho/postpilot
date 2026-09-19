import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `billing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'billing',
  ko: {
    nav: '결제 관리',
  },
  en: {
    nav: 'Billing',
  },
} as const satisfies I18nFragment
