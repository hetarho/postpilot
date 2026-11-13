import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    storylineSpace: {
      heading: '스토리라인',
      added: '이 스토리라인을 만든 뒤 사진이 추가됐어요. 다시 만들면 새 사진도 들어가요.',
    },
  },
  en: {
    storylineSpace: {
      heading: 'Storyline',
      added: 'Photos were added after this storyline was made. Make it again to include them.',
    },
  },
} as const satisfies I18nFragment
