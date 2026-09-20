import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `memories` namespace (ARCH-16). */
export const i18n = {
  namespace: 'memories',
  ko: {
    delete: {
      aria: '이 기억 삭제',
      title: '기억을 삭제할까요?',
      description: '되돌릴 수 없어요. 이미 시작된 생성은 시작할 때 고정된 기억을 그대로 씁니다.',
    },
  },
  en: {
    delete: {
      aria: 'Delete this memory',
      title: 'Delete this memory?',
      description:
        'This cannot be undone. Work already started keeps the memories frozen when it began.',
    },
  },
} as const satisfies I18nFragment
