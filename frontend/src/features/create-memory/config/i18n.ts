import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `memories` namespace (ARCH-16). */
export const i18n = {
  namespace: 'memories',
  ko: {
    create: {
      open: '새 기억',
      title: '기억 추가',
      text: '기억할 사실',
      textPlaceholder: '매운 음식을 잘 못 먹는다',
      help: '글 한 편에 없는, 다른 글에서도 그대로 참인 사실 하나를 적어 주세요.',
      submit: '저장',
      dockAria: '기억 추가',
    },
  },
  en: {
    create: {
      open: 'New memory',
      title: 'Add a memory',
      text: 'The fact to remember',
      textPlaceholder: 'Cannot handle spicy food',
      help: 'One fact that no single post carries and that stays true in the next one.',
      submit: 'Save',
      dockAria: 'Add a memory',
    },
  },
} as const satisfies I18nFragment
