import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16), under its own key group. The help line
 *  names what the choice feeds and nothing more: no exposure gain, and no claim that a whole post
 *  was read — the phrases come from search result titles and descriptions (QUAL-19). */
export const i18n = {
  namespace: 'posts',
  ko: {
    postField: {
      label: '분야',
      help: '다음 생성부터 이 분야의 지침과, 네이버 검색 결과의 제목·설명에서 자주 보인 표현을 함께 참고해요.',
    },
  },
  en: {
    postField: {
      // Not "Field": that already means a form field here.
      label: 'Category',
      help: 'From the next generation on, runs also use this category’s guidelines and the phrases seen most often in Naver search result titles and descriptions.',
    },
  },
} as const satisfies I18nFragment
