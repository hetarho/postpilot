import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    rename: {
      aria: '{{name}} 이름 바꾸기',
      label: '말투 이름',
      count: '{{count}} / {{max}}자',
      count_one: '{{count}} / {{max}}자',
      count_other: '{{count}} / {{max}}자',
    },
  },
  en: {
    rename: {
      aria: 'Rename {{name}}',
      label: 'Voice name',
      count: '{{count}} / {{max}} characters',
      count_one: '{{count}} / {{max}} character',
      count_other: '{{count}} / {{max}} characters',
    },
  },
} as const satisfies I18nFragment
