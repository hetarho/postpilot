import type { I18nFragment } from '@/shared/lib'

/** 홈: the promise, how ready the owner's writing kit is, and the one 글쓰기. */
export const i18n = {
  namespace: 'creation',
  ko: {
    home: {
      promise: { first: '고품질의 글을,', second: '내 언어로.' },
      status: {
        complete: '글 쓸 준비가 다 됐어요',
        partial: '{{total}}개 중 {{count}}개 준비됐어요',
        empty: '아직 채운 것이 없어요',
        loading: '준비 상태를 확인하고 있어요',
        failed: '준비 상태를 불러오지 못했어요',
      },
      write: '글쓰기',
      next: {
        voices: '말투 만들기',
        templates: '템플릿 만들기',
        guidelines: '지침 쓰기',
        memories: '기억 더하기',
      },
      actions: '홈에서 할 일',
    },
  },
  en: {
    home: {
      promise: { first: 'Quality writing,', second: 'in your own words.' },
      status: {
        complete: 'Ready to write',
        partial: '{{count}} of {{total}} ready',
        empty: 'Nothing filled in yet',
        loading: 'Checking your kit',
        failed: 'Could not load your kit',
      },
      write: 'Write',
      next: {
        voices: 'Make a style',
        templates: 'Make a template',
        guidelines: 'Write a rule',
        memories: 'Add a memory',
      },
      actions: 'Home actions',
    },
  },
} as const satisfies I18nFragment
