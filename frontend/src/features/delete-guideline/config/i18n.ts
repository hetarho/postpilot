import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16). */
export const i18n = {
  namespace: 'guidelines',
  ko: {
    delete: {
      aria: '지침 삭제',
      title: '이 지침을 삭제할까요?',
      description:
        '이 지침을 지웁니다. 이미 시작된 AI 작업은 시작할 때의 지침으로 끝나고, 글과 본문·템플릿·말투는 그대로예요.',
    },
  },
  en: {
    delete: {
      aria: 'Delete guideline',
      title: 'Delete this guideline?',
      description:
        'This removes the guideline. AI work already started finishes with the guidelines it started with, and your posts, templates and voices are untouched.',
    },
  },
} as const satisfies I18nFragment
