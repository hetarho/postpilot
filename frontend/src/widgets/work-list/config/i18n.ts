import type { I18nFragment } from '@/shared/lib'

/** Guide: 내 글's shared pieces — compact work cards, the status groups, the 찾기 sheet. */
export const i18n = {
  namespace: 'channel',
  ko: {
    work: {
      continueAction: '계속하기',
      newestFirst: '최근 수정 순',
      empty: '아직 만든 글이나 영상이 없어요. 만들기에서 시작해 보세요.',
      inProgress: '작성 중',
      done: '완성',
      find: '찾기',
      findTitle: '찾기',
      findActive: '찾기 · {{count}}',
      findDone: '결과 보기',
      clearFind: '찾기 지우기',
      kind: { post: '블로그', naverClip: '네이버 클립', instaClip: '인스타 클립' },
      more: '{{name}} 모두 보기',
    },
  },
  en: {
    work: {
      continueAction: 'Continue',
      newestFirst: 'Latest changes first',
      empty: 'No posts or videos yet. Start from Create.',
      inProgress: 'In progress',
      done: 'Finished',
      find: 'Find',
      findTitle: 'Find',
      findActive: 'Find · {{count}}',
      findDone: 'Show results',
      clearFind: 'Clear search',
      kind: { post: 'Blog', naverClip: 'Naver Clip', instaClip: 'Instagram Clip' },
      more: 'See all {{name}}',
    },
  },
} as const satisfies I18nFragment
