import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'channel',
  ko: {
    menu: {
      naverBlog: '네이버 블로그 메뉴',
      naverClip: '네이버 클립 메뉴',
      instaClip: '인스타 클립 메뉴',
    },
    newPost: '새 글',
    newClip: '새 클립',
    close: '메뉴 닫기',
    createPost: '새 글 쓰기',
    createClip: '새 클립 만들기',
    postTemplates: '글 템플릿',
    postGuidelines: '글 지침',
    videoTemplates: '영상 템플릿',
    videoGuidelines: '영상 지침',
    voices: '말투',
  },
  en: {
    menu: {
      naverBlog: 'Naver Blog menu',
      naverClip: 'Naver Clip menu',
      instaClip: 'Instagram Clip menu',
    },
    newPost: 'New post',
    newClip: 'New clip',
    close: 'Close menu',
    createPost: 'Write a new post',
    createClip: 'Create a new clip',
    postTemplates: 'Post templates',
    postGuidelines: 'Post guidelines',
    videoTemplates: 'Video templates',
    videoGuidelines: 'Video guidelines',
    voices: 'Writing voices',
  },
} as const satisfies I18nFragment
