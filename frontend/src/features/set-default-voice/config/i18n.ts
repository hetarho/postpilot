import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    setDefault: { action: '기본으로 설정' },
  },
  en: {
    setDefault: { action: 'Make default' },
  },
} as const satisfies I18nFragment
