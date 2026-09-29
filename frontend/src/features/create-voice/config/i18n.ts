import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    create: {
      open: '새 말투 만들기',
      dockAria: '말투 추가',
      title: '새 말투',
      name: '말투 이름',
      placeholder: '예: 제품 리뷰',
      submit: '말투 만들기',
      count: '{{count}} / {{max}}자',
      count_one: '{{count}} / {{max}}자',
      count_other: '{{count}} / {{max}}자',
    },
  },
  en: {
    create: {
      open: 'New voice',
      dockAria: 'Add a voice',
      title: 'New voice',
      name: 'Voice name',
      placeholder: 'For example: Product reviews',
      submit: 'Create voice',
      count: '{{count}} / {{max}} characters',
      count_one: '{{count}} / {{max}} character',
      count_other: '{{count}} / {{max}} characters',
    },
  },
} as const satisfies I18nFragment
