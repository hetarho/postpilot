import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16), under its own key group. The field
 *  carries its name and nothing else (owner decision 2026-09-25). */
export const i18n = {
  namespace: 'posts',
  ko: {
    postField: {
      label: '분야',
    },
  },
  en: {
    postField: {
      // Not "Field": that already means a form field here.
      label: 'Category',
    },
  },
} as const satisfies I18nFragment
