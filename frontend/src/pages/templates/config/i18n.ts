import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `templates` namespace (ARCH-16). */
export const i18n = {
  namespace: 'templates',
  ko: {
    page: {
      description:
        '템플릿은 글의 구성과 순서를 정해요. 글마다 하나를 고르면 AI가 그 구성대로 씁니다. 문체와 종결어미는 그대로 말투 프로필을 따라요.',
      lead: '글이 이 순서로 짜여요',
      saved: '저장된 템플릿',
      new: '새 템플릿',
      empty: '아직 저장된 템플릿이 없어요',
      emptyHelp:
        '어떤 글을 올리는지 AI에게 말해 주세요. 처음부터 틀을 짜 드리고, 대화하며 고쳐 저장해요. 글을 쓸 때 골라 쓰면 AI가 매번 그 순서대로 써요.',
      sub: 'AI와 대화해서 만들고, 글을 쓸 때 골라 써요.',
      name: '이름',
      newDockAria: '새 템플릿 만들기',
    },
  },
  en: {
    page: {
      description:
        'A template decides the structure and order of a post. Choose one per post and AI follows that structure, while the voice still controls style and endings.',
      lead: 'Your posts follow this order',
      saved: 'Saved templates',
      new: 'New template',
      empty: 'There are no saved templates yet',
      emptyHelp:
        'Tell the AI what you post. It drafts the shape from scratch, you refine it by talking and save it. Pick it when you write and the AI follows that order every time.',
      sub: 'Made by talking to the AI, picked when you write.',
      name: 'Name',
      newDockAria: 'Create a new template',
    },
  },
} as const satisfies I18nFragment
