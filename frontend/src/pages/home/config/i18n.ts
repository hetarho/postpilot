import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'creation',
  ko: {
    home: {
      heading: '내가 겪은 것만으로,',
      emphasis: '채널에 맞게 써 드려요.',
      intro: '사진과 영상, 남겨 둔 메모로 시작하세요.',
      choices: '채널 선택',
      naverBlog: '네이버 블로그',
      naverBlogDescription: '사진과 메모로 내 말투의 글을 써요',
      naverClip: '네이버 클립',
      naverClipDescription: '체험 영상을 네이버 클립으로 만들어요',
      instaClip: '인스타 클립',
      instaClipDescription: '체험 영상을 인스타 클립으로 만들어요',
    },
  },
  en: {
    home: {
      heading: 'Your own experience.',
      emphasis: 'Written for your channel.',
      intro: 'Start with your photos, videos and notes.',
      choices: 'Choose a channel',
      naverBlog: 'Naver Blog',
      naverBlogDescription: 'Write from photos and notes in your own voice',
      naverClip: 'Naver Clip',
      naverClipDescription: 'Make a Naver clip from your experience',
      instaClip: 'Instagram Clip',
      instaClipDescription: 'Make an Instagram clip from your experience',
    },
  },
} as const satisfies I18nFragment
