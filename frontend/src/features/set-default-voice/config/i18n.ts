import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    setDefault: { action: '기본으로 설정', clear: '기본 해제' },
  },
  en: {
    setDefault: { action: 'Make default', clear: 'Remove default' },
  },
} as const satisfies I18nFragment
