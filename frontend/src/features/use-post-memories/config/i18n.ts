import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). Its own key group: `editor` belongs to
 *  the editor page's fragment, and two fragments may not claim one key. */
export const i18n = {
  namespace: 'posts',
  ko: {
    useMemories: {
      label: '기억 사용',
    },
  },
  en: {
    useMemories: {
      label: 'Use memories',
    },
  },
} as const satisfies I18nFragment
