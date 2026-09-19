import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16). */
export const i18n = {
  namespace: 'guidelines',
  ko: {
    scope: {
      label: '적용 범위',
      global: '전역',
      templates: '특정 템플릿',
      globalHelp: '이 계정의 모든 글에 적용돼요.',
      templatesHelp: '고른 템플릿이 지정된 글에만 적용돼요.',
      pick: '적용할 템플릿',
      orphaned: '적용 대상 없음',
      orphanedHelp:
        '지정했던 템플릿이 삭제돼서 지금은 어떤 글에도 적용되지 않아요. 범위를 다시 고르거나 삭제해 주세요.',
      templatesEmpty: '먼저 템플릿을 하나 만들어 주세요.',
    },
    create: {
      open: '새 지침',
      dockAria: '지침 추가',
      title: '새 지침',
      text: '지침',
      textPlaceholder: '예: 무인 매장 글에서 CCTV를 언급하지 않기',
      help: '한 줄에 규칙 하나씩, 짧게 적어 주세요. 지침이 템플릿의 요구와 충돌하면 지침을 우선합니다.',
      submit: '지침 만들기',
    },
  },
  en: {
    scope: {
      label: 'Applies to',
      global: 'Everything',
      templates: 'Specific templates',
      globalHelp: 'Applies to every post of this account.',
      templatesHelp: 'Applies only to posts assigned one of the templates you pick.',
      pick: 'Templates',
      orphaned: 'Applies to nothing',
      orphanedHelp:
        'Every template this was scoped to has been deleted, so it currently reaches no post. Pick a scope again, or delete it.',
      templatesEmpty: 'Create a template first.',
    },
    create: {
      open: 'New guideline',
      dockAria: 'Add a guideline',
      title: 'New guideline',
      text: 'Guideline',
      textPlaceholder: 'e.g. In unmanned-store posts, do not mention CCTV',
      help: 'One short rule per guideline. A guideline wins over a conflicting template instruction.',
      submit: 'Create guideline',
    },
  },
} as const satisfies I18nFragment
