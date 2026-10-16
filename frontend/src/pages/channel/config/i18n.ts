import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'channel',
  ko: {
    naverBlog: '네이버 블로그',
    naverClip: '네이버 클립',
    instaClip: '인스타 클립',
    createPost: '새 글 쓰기',
    createClip: '새 클립 만들기',
    history: '기록',
    templates: '템플릿',
    guidelines: '지침',
  },
  en: {
    naverBlog: 'Naver Blog',
    naverClip: 'Naver Clip',
    instaClip: 'Instagram Clip',
    createPost: 'Write a new post',
    createClip: 'Create a new clip',
    history: 'History',
    templates: 'Templates',
    guidelines: 'Guidelines',
  },
} as const satisfies I18nFragment
