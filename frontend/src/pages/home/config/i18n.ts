import type { I18nFragment } from '@/shared/lib'

/** 홈: the promise, how ready the owner's writing kit is, and the one 글쓰기. */
export const i18n = {
  namespace: 'creation',
  ko: {
    home: {
      promise: { first: '고품질의 글을,', second: '내 언어로.' },
      failed: '준비 상태를 불러오지 못했어요',
      write: '글쓰기',
      next: {
        voices: '말투 만들기',
        templates: '템플릿 만들기',
        guidelines: '지침 쓰기',
        memories: '기억 더하기',
      },
      resume: { voices: '말투 마저 만들기', templates: '만들던 템플릿 잇기' },
      actions: '홈에서 할 일',
    },
  },
  en: {
    home: {
      promise: { first: 'Quality writing,', second: 'in your own words.' },
      failed: 'Could not load your kit',
      write: 'Write',
      next: {
        voices: 'Make a style',
        templates: 'Make a template',
        guidelines: 'Write a rule',
        memories: 'Add a memory',
      },
      resume: { voices: 'Finish your style', templates: 'Resume a template' },
      actions: 'Home actions',
    },
  },
} as const satisfies I18nFragment
