import type { I18nFragment } from '@/shared/lib'

/** Guide: 만들기 — the tab that asks one question. */
export const i18n = {
  namespace: 'creation',
  ko: {
    guideCreate: {
      title: '무엇을 만들까요?',
      choices: '만들 것',
      post: '글',
      postOutcome: '사진과 메모로 블로그 글 한 편',
      video: '영상',
      videoOutcome: '세로 영상, 자막과 목소리까지',
    },
  },
  en: {
    guideCreate: {
      title: 'What shall we make?',
      choices: 'Things to make',
      post: 'Post',
      postOutcome: 'A blog post from your photos and notes',
      video: 'Video',
      videoOutcome: 'A vertical video with captions and a voice',
    },
  },
} as const satisfies I18nFragment
