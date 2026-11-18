import type { I18nFragment } from '@/shared/lib'

export const i18n = {
  namespace: 'clips',
  ko: {
    history: {
      videos: '영상',
      empty: '아직 영상이 없어요. 아래 + 새 영상에서 만들 수 있어요.',
      download: '영상 다운로드',
    },
  },
  en: {
    history: {
      videos: 'Videos',
      empty: 'No videos yet. Make one from + New video below.',
      download: 'Download video',
    },
  },
} as const satisfies I18nFragment
