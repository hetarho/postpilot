import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'channel',
  ko: {
    button: {
      work: '새 글',
      video: '새 영상',
      templates: '새 템플릿',
      voices: '새 말투',
      guidelines: '새 지침',
    },
  },
  en: {
    button: {
      work: 'New post',
      video: 'New video',
      templates: 'New template',
      voices: 'New voice',
      guidelines: 'New guideline',
    },
  },
} as const satisfies I18nFragment
