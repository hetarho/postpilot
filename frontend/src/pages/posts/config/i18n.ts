import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    new: '새 글',
    list: {
      mine: '글',
      loadFailed: '목록을 불러오지 못했어요.',
      empty: '아직 글이 없어요. 아래 + 새 글에서 시작할 수 있어요.',
      writingAria: '글 작성',
      history: {
        directory: '글 작업 내역',
        exportReady: '내보내기 가능',
        published: '발행한 글',
        result: '이전 AI 결과',
      },
      search: '검색',
      searchPlaceholder: '제목 또는 태그',
      filter: {
        label: '상태',
        all: '전체',
        draft: '초안',
        review: '검토',
        finalized: '확정',
        published: '발행됨',
      },
      noMatch: {
        query: '"{{q}}"에 맞는 글이 없어요.',
        status: '{{status}} 상태인 글이 없어요.',
        both: '{{status}} 상태에서 "{{q}}"에 맞는 글이 없어요.',
      },
      reset: '초기화',
      loadingMore: '불러오는 중…',
      loadMoreFailed: '더 불러오지 못했어요.',
    },
  },
  en: {
    new: 'New post',
    list: {
      mine: 'Posts',
      loadFailed: 'Could not load the post list.',
      empty: 'There are no posts yet. Start one from + New post below.',
      writingAria: 'Write a post',
      history: {
        directory: 'Writing history',
        exportReady: 'Ready to export',
        published: 'Published post',
        result: 'Previous AI result',
      },
      search: 'Search',
      searchPlaceholder: 'Title or tag',
      filter: {
        label: 'Status',
        all: 'All',
        draft: 'Draft',
        review: 'Review',
        finalized: 'Finalized',
        published: 'Published',
      },
      noMatch: {
        query: 'No post matches "{{q}}".',
        status: 'No post is in {{status}}.',
        both: 'No {{status}} post matches "{{q}}".',
      },
      reset: 'Clear',
      loadingMore: 'Loading more…',
      loadMoreFailed: 'Could not load more.',
    },
  },
} as const satisfies I18nFragment
