import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'creation',
  ko: {
    library: {
      title: '작업 내역',
      description: '진행 중인 작업과 실패한 시도를 확인하고, 이어서 작업하거나 결과를 내보내세요.',
      posts: '글 작업 내역',
      postsDescription: '작성 상태, AI 결과와 글 내보내기',
      clips: '클립 작업 내역',
      clipsDescription: '편집 상태, 실패한 시도와 영상 다운로드',
    },
  },
  en: {
    library: {
      title: 'Work history',
      description: 'Check ongoing work and failed attempts, continue working, or export a result.',
      posts: 'Writing history',
      postsDescription: 'Writing status, AI results and post exports',
      clips: 'Clip history',
      clipsDescription: 'Editing status, failed attempts and video downloads',
    },
  },
} as const satisfies I18nFragment
