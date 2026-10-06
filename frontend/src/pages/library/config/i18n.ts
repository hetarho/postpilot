import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'creation',
  ko: {
    library: {
      title: '보관함',
      description: '전에 만든 작업을 이어가거나 완성된 작품을 찾아보세요.',
      posts: '내 글',
      postsDescription: '작성 중인 글과 완성한 글',
      clips: '내 클립',
      clipsDescription: '편집 중인 클립과 완성한 영상',
    },
  },
  en: {
    library: {
      title: 'Saved work',
      description: 'Pick up a draft or revisit something you made.',
      posts: 'My posts',
      postsDescription: 'Writing in progress and finished posts',
      clips: 'My clips',
      clipsDescription: 'Clips in progress and finished videos',
    },
  },
} as const satisfies I18nFragment
