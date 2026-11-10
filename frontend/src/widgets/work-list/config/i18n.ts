import type { I18nFragment } from '@/shared/lib'

/** Guide: 내 글's shared pieces — the 이어서 쓰기 hero, the status groups, the 찾기 sheet. */
export const i18n = {
  namespace: 'channel',
  ko: {
    work: {
      continue: '이어서 쓰기',
      continueAction: '계속하기',
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
      continue: 'Pick up where you left off',
      continueAction: 'Continue',
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
