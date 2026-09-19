import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `templates` namespace (ARCH-16). */
export const i18n = {
  namespace: 'templates',
  ko: {
    delete: {
      aria: '{{name}} 삭제',
      title: '이 템플릿을 삭제할까요?',
      description:
        '‘{{name}}’을(를) 지웁니다. {{detach}} 이미 만들어진 글의 결과와 진행 중인 작업은 그대로예요.',
    },
    postCount: '글 {{count}}개',
  },
  en: {
    delete: {
      aria: 'Delete {{name}}',
      title: 'Delete this template?',
      description:
        '‘{{name}}’ will be removed. {{detach}} Posts that were already generated and work in progress are unaffected.',
    },
    postCount: '{{count}} posts',
  },
} as const satisfies I18nFragment
