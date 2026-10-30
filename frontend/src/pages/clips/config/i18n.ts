import type { I18nFragment } from '@/shared/lib'

export const i18n = {
  namespace: 'clips',
  ko: {
    history: {
      naverClip: '네이버 클립',
      instaClip: '인스타 클립',
      empty: '아직 클립이 없어요. 아래 + 새 클립에서 만들 수 있어요.',
      continue: '이어서 편집',
      open: '작업 확인',
      download: '영상 다운로드',
      export: '영상 내보내기',
      untitled: '제목 없는 클립',
    },
  },
  en: {
    history: {
      naverClip: 'Naver Clip',
      instaClip: 'Instagram Clip',
      empty: 'There are no clips yet. Create one from + New clip below.',
      continue: 'Continue editing',
      open: 'View work',
      download: 'Download video',
      export: 'Export video',
      untitled: 'Untitled clip',
    },
  },
} as const satisfies I18nFragment
