import type { I18nFragment } from '@/shared/lib'

export const i18n = {
  namespace: 'clips',
  ko: {
    history: {
      videos: '영상',
      empty: '아직 만든 영상이 없어요',
      emptyHelp: '찍어 둔 사진과 영상으로 AI가 짧은 영상을 엮어 줘요.',
      continue: '이어서 편집',
      open: '작업 확인',
      download: '영상 다운로드',
      export: '영상 내보내기',
    },
  },
  en: {
    history: {
      videos: 'Videos',
      empty: 'No videos yet',
      emptyHelp: 'The AI cuts the photos and footage you took into a short video.',
      continue: 'Continue editing',
      open: 'View work',
      download: 'Download video',
      export: 'Export video',
    },
  },
} as const satisfies I18nFragment
