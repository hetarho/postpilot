import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    undo: {
      action: '이전 분석으로 되돌리기',
      title: '이전 분석으로 되돌릴까요?',
      description:
        '지금 분석은 지워지고 바로 전 분석이 다시 쓰여요. 되돌린 뒤에는 다시 앞으로 돌아갈 수 없어요.',
    },
  },
  en: {
    undo: {
      action: 'Return to the previous analysis',
      title: 'Return to the previous analysis?',
      description:
        'The current analysis is discarded and the one before it is used again. This cannot be redone.',
    },
  },
} as const satisfies I18nFragment
