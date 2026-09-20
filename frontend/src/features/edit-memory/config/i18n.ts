import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `memories` namespace (ARCH-16). */
export const i18n = {
  namespace: 'memories',
  ko: {
    edit: {
      text: '기억',
      facets: '종류와 태그',
      noTags: '태그 없음',
    },
  },
  en: {
    edit: {
      text: 'Memory',
      facets: 'Kind and tags',
      noTags: 'No tags',
    },
  },
} as const satisfies I18nFragment
