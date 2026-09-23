import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    new: '새 글',
    list: {
      mine: '내 글',
      loadFailed: '목록을 불러오지 못했어요.',
      empty: '아직 글이 없어요. "새 글"로 시작해 보세요.',
      writingAria: '글 작성',
      state: { generating: 'AI 생성 중', failed: 'AI 결과 오류', review: 'AI 결과 확인' },
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
    },
  },
  en: {
    new: 'New post',
    list: {
      mine: 'My posts',
      loadFailed: 'Could not load the post list.',
      empty: 'There are no posts yet. Start with "New post".',
      writingAria: 'Write a post',
      state: { generating: 'AI generation', failed: 'AI result error', review: 'Review AI result' },
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
    },
  },
} as const satisfies I18nFragment
