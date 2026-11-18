import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    setDefault: { action: '기본으로 설정', clear: '기본 해제' },
    defaultChoice: { empty: '아직 만든 말투가 없어요. 말투 화면에서 만들 수 있어요.' },
  },
  en: {
    setDefault: { action: 'Make default', clear: 'Remove default' },
    defaultChoice: {
      empty: 'No writing style yet. You can make one on the writing styles screen.',
    },
  },
} as const satisfies I18nFragment
