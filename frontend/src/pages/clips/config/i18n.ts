import type { I18nFragment } from '@/shared/lib'

export const i18n = {
  namespace: 'clips',
  ko: {
    history: {
      title: '클립 작업 내역',
      continue: '이어서 편집',
      open: '작업 확인',
      download: '영상 다운로드',
      export: '영상 내보내기',
      untitled: '제목 없는 클립',
    },
  },
  en: {
    history: {
      title: 'Clip history',
      continue: 'Continue editing',
      open: 'View work',
      download: 'Download video',
      export: 'Export video',
      untitled: 'Untitled clip',
    },
  },
} as const satisfies I18nFragment
