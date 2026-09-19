import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `templates` namespace (ARCH-16). */
export const i18n = {
  namespace: 'templates',
  ko: {
    assignment: {
      runningJob:
        '진행 중인 AI 작업은 시작할 때의 템플릿으로 끝나요. 바꾼 템플릿은 다음 생성부터 적용됩니다.',
      notFound: '고른 템플릿을 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요.',
      notFoundDetail:
        '고른 템플릿을 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요. {{error}}',
      failed: '템플릿을 바꾸지 못했어요. 다시 시도해 주세요.',
    },
  },
  en: {
    assignment: {
      runningJob:
        'A running AI job finishes with the template it started with. A change applies from the next generation.',
      notFound: 'Could not find the selected template. Refresh the list and try again.',
      notFoundDetail:
        'Could not find the selected template. Refresh the list and try again. {{error}}',
      failed: 'Could not change the template. Please try again.',
    },
  },
} as const satisfies I18nFragment
