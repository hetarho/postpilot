import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'channel',
  ko: {
    menu: {
      naverBlog: '네이버 블로그 만들기',
      naverClip: '네이버 클립 만들기',
      instaClip: '인스타 클립 만들기',
    },
    button: {
      work: '새 글',
      video: '새 영상',
      templates: '새 템플릿',
      voices: '새 말투',
      guidelines: '새 지침',
    },
    create: {
      post: '새 글쓰기',
      video: '새 영상 만들기',
      template: '새 템플릿',
      voice: '새 말투',
      guideline: '새 지침',
    },
    close: '메뉴 닫기',
  },
  en: {
    menu: {
      naverBlog: 'Create for Naver Blog',
      naverClip: 'Create for Naver Clip',
      instaClip: 'Create for Instagram Clip',
    },
    button: {
      work: 'New post',
      video: 'New video',
      templates: 'New template',
      voices: 'New voice',
      guidelines: 'New guideline',
    },
    create: {
      post: 'Write a new post',
      video: 'Create a new video',
      template: 'New template',
      voice: 'New voice',
      guideline: 'New guideline',
    },
    close: 'Close menu',
  },
} as const satisfies I18nFragment
