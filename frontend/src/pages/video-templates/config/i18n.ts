import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    directory: {
      title: '영상 템플릿',
      description: '필요한 정보와 컷 구성, 영상 디자인을 저장해 두세요.',
      saved: '저장된 영상 템플릿',
      empty: '아직 저장된 영상 템플릿이 없어요',
      emptyHelp:
        '클립을 만들 때 받을 정보와 컷 구성, 영상 디자인을 한 번 정해 두면 매번 다시 정하지 않아도 돼요.',
      newDockAria: '새 영상 템플릿 만들기',
      loadFailed: '영상 템플릿을 불러오지 못했어요.',
      projectCount: '클립 {{count}}개',
      create: '새 영상 템플릿',
      fields: '정보 {{count}}개',
      updated: '수정 {{date}}',
    },
  },
  en: {
    directory: {
      title: 'Video templates',
      description: 'Save the information to collect, cut guidance and video design.',
      saved: 'Saved video templates',
      empty: 'No video templates yet',
      emptyHelp:
        'Decide once what a clip should ask for, how its cuts are composed and how its copy looks, and every clip reuses it.',
      newDockAria: 'Create a new video template',
      loadFailed: 'Could not load your video templates.',
      projectCount: '{{count}} clips',
      create: 'New video template',
      fields: '{{count}} information fields',
      updated: 'Updated {{date}}',
    },
  },
} as const satisfies I18nFragment
