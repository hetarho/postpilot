import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16), under its own key group. The phrases
 *  are described by where they were observed, never by what they gain, and nothing here says a
 *  post was read (QUAL-19, QUAL-21). */
export const i18n = {
  namespace: 'posts',
  ko: {
    replacements: {
      panel: '‘{{source}}’ 바꿔 쓰기',
      source: '‘{{source}}’ 대신 쓸 수 있는 표현',
      take: '‘{{phrase}}’(으)로 바꾸기',
      observed: '네이버 검색 결과의 제목과 설명에 자주 나온 표현이에요.',
    },
  },
  en: {
    replacements: {
      panel: 'Rewrite “{{source}}”',
      source: 'Phrases for “{{source}}”',
      take: 'Replace with “{{phrase}}”',
      observed: 'These phrases appear often in Naver search result titles and descriptions.',
    },
  },
} as const satisfies I18nFragment
