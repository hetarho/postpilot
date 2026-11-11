import type { I18nFragment } from '@/shared/lib'

/** Guide: 만들기 — the tab that asks one question. */
export const i18n = {
  namespace: 'creation',
  ko: {
    guideCreate: {
      title: '무엇을 만들까요?',
      choices: '만들 것',
      blog: '블로그 글',
      blogOutcome: '사진과 메모로 네이버 블로그 글 한 편',
      naverClip: '네이버 클립',
      naverClipOutcome: '세로 영상, 자막과 목소리까지',
      instaClip: '인스타 클립',
      instaClipOutcome: '릴스로 올릴 짧은 영상',
    },
  },
  en: {
    guideCreate: {
      title: 'What shall we make?',
      choices: 'Things to make',
      blog: 'Blog post',
      blogOutcome: 'A Naver Blog post from your photos and notes',
      naverClip: 'Naver Clip',
      naverClipOutcome: 'A vertical video with captions and a voice',
      instaClip: 'Instagram Clip',
      instaClipOutcome: 'A short video for Reels',
    },
  },
} as const satisfies I18nFragment
