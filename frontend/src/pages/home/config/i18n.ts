import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'creation',
  ko: {
    home: {
      heading: '당신의 이야기를,',
      emphasis: '새로운 작품으로.',
      intro: '오늘은 무엇을 만들어 볼까요?',
      post: '새 글 작성하기',
      postDescription: '사진과 메모를 나다운 글로',
      clip: '새 클립 만들기',
      clipDescription: '일상의 장면을 하나의 영상으로',
      choices: '새로 만들기',
    },
  },
  en: {
    home: {
      heading: 'Your story.',
      emphasis: 'Something new.',
      intro: 'What will you create today?',
      post: 'Write a new post',
      postDescription: 'Turn photos and notes into your words',
      clip: 'Create a new clip',
      clipDescription: 'Bring your moments together in a video',
      choices: 'Create something new',
    },
  },
} as const satisfies I18nFragment
