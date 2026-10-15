import type { I18nFragment } from '@/shared/lib'

export const headingErrors = {
  namespace: 'errors',
  ko: { TEMPLATE_HEADING_INPUT_INVALID: '제목을 제안받을 부분의 이름과 설명을 확인해 주세요.' },
  en: {
    TEMPLATE_HEADING_INPUT_INVALID:
      'Check the names and descriptions of the parts to suggest headings for.',
  },
} as const satisfies I18nFragment
