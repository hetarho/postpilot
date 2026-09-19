import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    delete: {
      title: '영상 템플릿을 삭제할까요?',
      description:
        '클립 {{count}}개에서 이 템플릿의 연결이 해제돼요. 답변과 생성된 영상은 보존돼요.',
      action: '삭제',
      aria: '{{name}} 삭제',
    },
  },
  en: {
    delete: {
      title: 'Delete this video template?',
      description:
        'This template will be detached from {{count}} clips. Answers and generated videos will be kept.',
      action: 'Delete',
      aria: 'Delete {{name}}',
    },
  },
} as const satisfies I18nFragment
