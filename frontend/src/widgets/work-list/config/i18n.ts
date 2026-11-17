import type { I18nFragment } from '@/shared/lib'

/** Guide: 내 글's shared pieces — compact work cards, the status groups, the 찾기 sheet. */
export const i18n = {
  namespace: 'channel',
  ko: {
    heading: {
      posts: { title: '글', lead: '쓰고 있는 블로그 글과 완성한 글' },
      videos: { title: '영상', lead: '만들고 있는 세로 영상과 완성한 영상' },
    },
    work: {
      continueAction: '계속하기',
      newestFirst: '최근 수정 순',
      empty: '아직 만든 글이나 영상이 없어요. 홈의 글쓰기에서 시작해 보세요.',
      inProgress: '작성 중',
      done: '완성',
      find: '찾기',
      findTitle: '찾기',
      findActive: '찾기 · {{count}}',
      findDone: '결과 보기',
      clearFind: '찾기 지우기',
      kind: { post: '글', video: '영상' },
      more: '{{name}} 모두 보기',
      search: '제목',
      searchPlaceholder: '제목으로 찾기',
      kindFilter: { label: '종류', all: '전체' },
      statusFilter: { label: '상태', all: '전체', inProgress: '작성 중', done: '완성' },
      sort: { label: '정렬', updated: '최근 수정 순', oldest: '오래된 순', title: '제목 순' },
      noMatch: '조건에 맞는 콘텐츠가 없어요.',
    },
  },
  en: {
    heading: {
      posts: { title: 'Posts', lead: 'Blog posts you are writing and posts you finished' },
      videos: { title: 'Videos', lead: 'Vertical videos you are making and videos you finished' },
    },
    work: {
      continueAction: 'Continue',
      newestFirst: 'Latest changes first',
      empty: 'No posts or videos yet. Start from Write on Home.',
      inProgress: 'In progress',
      done: 'Finished',
      find: 'Find',
      findTitle: 'Find',
      findActive: 'Find · {{count}}',
      findDone: 'Show results',
      clearFind: 'Clear search',
      kind: { post: 'Post', video: 'Video' },
      more: 'See all {{name}}',
      search: 'Title',
      searchPlaceholder: 'Find by title',
      kindFilter: { label: 'Kind', all: 'All' },
      statusFilter: { label: 'Status', all: 'All', inProgress: 'In progress', done: 'Finished' },
      sort: {
        label: 'Sort',
        updated: 'Latest changes first',
        oldest: 'Oldest first',
        title: 'By title',
      },
      noMatch: 'Nothing matches.',
    },
  },
} as const satisfies I18nFragment
