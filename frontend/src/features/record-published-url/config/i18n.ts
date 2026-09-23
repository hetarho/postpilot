import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16), under its own key group. */
export const i18n = {
  namespace: 'posts',
  ko: {
    publishedUrl: {
      title: '발행',
      label: '네이버 블로그 글 주소',
      placeholder: 'https://blog.naver.com/…',
      help: '네이버 블로그에 올린 이 글의 주소를 붙여 넣으면 발행됨으로 바뀌어요. 발행된 글은 고칠 수 없고, 주소를 지우면 확정 상태로 돌아가요.',
      notFinalized: '글을 확정하면 발행 URL을 입력할 수 있어요.',
      clear: '지우기',
    },
  },
  en: {
    publishedUrl: {
      title: 'Publish',
      label: 'Naver Blog post address',
      placeholder: 'https://blog.naver.com/…',
      help: 'Paste this post’s Naver Blog address to mark it Published. A published post cannot be changed; clearing the address returns it to Finalized.',
      notFinalized: 'This field opens once the post is finalized.',
      clear: 'Clear',
    },
  },
} as const satisfies I18nFragment
