import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16), under its one key `preset`. The
 *  phrases are described only by what is observed about them — where they appear — never by what
 *  they gain, and nothing here says a post was read (QUAL-19, QUAL-21). */
export const i18n = {
  namespace: 'guidelines',
  ko: {
    preset: {
      name: '상위 노출 단어 사용',
      about:
        '고른 분야의 네이버 검색 상위 글 제목과 설명에 자주 나오는 문구를, 원문이 이미 같은 뜻으로 쓴 자리에서만 그 문구로 바꿔 쓰게 해요.',
      yields: '내가 저장한 지침과 부딪치면 항상 내 지침을 따라요.',
      fields: '적용할 분야',
      needsField:
        '켜져 있지만 고른 분야가 없어서 어떤 글에도 적용되지 않아요. 적용할 분야를 하나 골라 주세요.',
    },
  },
  en: {
    preset: {
      name: 'Top-result phrases',
      about:
        'For the categories you pick, it lets the writer use phrases that often appear in the titles and descriptions of top Naver search results, only where your source already says the same thing.',
      yields: 'When it conflicts with one of your own guidelines, yours wins.',
      fields: 'Categories',
      needsField:
        'It is on, but no category is picked, so it applies to no post. Pick a first category.',
    },
  },
} as const satisfies I18nFragment
