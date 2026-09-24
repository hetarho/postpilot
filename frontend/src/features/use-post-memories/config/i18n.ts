import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). Its own key group: `editor` belongs to
 *  the editor page's fragment, and two fragments may not claim one key. */
export const i18n = {
  namespace: 'posts',
  ko: {
    useMemories: {
      label: '기억 사용',
      saveFailed: '설정을 저장하지 못했어요. 다시 눌러 주세요.',
    },
  },
  en: {
    useMemories: {
      label: 'Use memories',
      saveFailed: 'Could not save the option. Press it again.',
    },
  },
} as const satisfies I18nFragment
