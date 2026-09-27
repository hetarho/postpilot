import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    directory: {
      title: '영상 템플릿',
      description: '클립에 받을 정보, 순서대로 담을 구성과 시작 디자인을 저장해 두세요.',
      saved: '저장된 영상 템플릿',
      empty: '아직 저장된 영상 템플릿이 없어요',
      emptyHelp:
        '받을 정보와 구성의 순서, 시작 디자인을 한 번 정해 두면 이 템플릿을 고른 클립이 그대로 시작해요.',
      newDockAria: '새 영상 템플릿 만들기',
      loadFailed: '영상 템플릿을 불러오지 못했어요.',
      projectCount: '클립 {{count}}개',
      create: '새 영상 템플릿',
      updated: '수정 {{date}}',
    },
  },
  en: {
    directory: {
      title: 'Video templates',
      description:
        'Save what a clip asks for, the outline it follows in order and the design it starts in.',
      saved: 'Saved video templates',
      empty: 'No video templates yet',
      emptyHelp:
        'Decide once what a clip asks for, the order it follows and the design it starts in, and a clip that picks this template starts that way.',
      newDockAria: 'Create a new video template',
      loadFailed: 'Could not load your video templates.',
      projectCount: '{{count}} clips',
      create: 'New video template',
      updated: 'Updated {{date}}',
    },
  },
} as const satisfies I18nFragment
