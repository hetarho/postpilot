import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `plans` namespace (ARCH-16). */
export const i18n = {
  namespace: 'plans',
  ko: {
    badge: {
      label: '플랜 {{tier}}, 남은 크레딧 {{count}}',
      labelUnlimited: '플랜 {{tier}}, 크레딧 차감 면제',
    },
  },
  en: {
    badge: {
      label: 'Plan {{tier}}, {{count}} credits left',
      labelUnlimited: 'Plan {{tier}}, credit debit exempt',
    },
  },
} as const satisfies I18nFragment
