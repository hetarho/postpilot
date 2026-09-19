import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `templates` namespace (ARCH-16). */
export const i18n = {
  namespace: 'templates',
  ko: {
    page: {
      description:
        '템플릿은 글의 구성과 순서를 정해요. 글마다 하나를 고르면 AI가 그 구성대로 씁니다. 문체와 종결어미는 그대로 말투 프로필을 따라요.',
      saved: '저장된 템플릿',
      new: '새 템플릿',
      empty: '아직 저장된 템플릿이 없어요',
      emptyHelp:
        '매번 같은 틀로 쓰는 글이 있다면, 그 틀을 한 번만 만들어 두고 글마다 골라 쓰세요. 인트로·사진 설명·총평처럼 글의 순서를 정해 두면 AI가 매번 그 순서대로 씁니다.',
      name: '이름',
      newDockAria: '새 템플릿 만들기',
    },
  },
  en: {
    page: {
      description:
        'A template decides the structure and order of a post. Choose one per post and AI follows that structure, while the voice still controls style and endings.',
      saved: 'Saved templates',
      new: 'New template',
      empty: 'There are no saved templates yet',
      emptyHelp:
        'If you write the same shape of post again and again, build that shape once and pick it per post. Set the order — intro, a description per photo, a verdict — and AI follows it every time.',
      name: 'Name',
      newDockAria: 'Create a new template',
    },
  },
} as const satisfies I18nFragment
