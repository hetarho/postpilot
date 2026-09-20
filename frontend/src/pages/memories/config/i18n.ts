import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `memories` namespace (ARCH-16). */
export const i18n = {
  namespace: 'memories',
  ko: {
    title: '기억',
    loadFailed: '기억을 불러오지 못했어요.',
    page: {
      description:
        '글 한 편에 담기지 않는, 글쓴이에 대한 사실입니다. ① 에서 기억 사용을 켠 글만 이 사실들을 함께 씁니다.',
      saved: '저장된 기억',
      order: '최근에 쓰인 기억이 먼저 옵니다. 글에 들어가는 순서도 같아요.',
      empty: '아직 기억이 없어요',
      emptyHelp:
        '글을 완성한 뒤 ③에서 기억으로 저장을 눌러 뽑아내거나, 여기서 직접 적을 수 있어요.',
      example: '예: 매운 음식을 잘 못 먹는다',
    },
  },
  en: {
    title: 'Memories',
    loadFailed: 'Could not load your memories.',
    page: {
      description:
        'Facts about you that no single post carries. Only a post with 기억 사용 turned on in ① is written with them.',
      saved: 'Saved memories',
      order: 'Most recently used first — the same order a post is given them in.',
      empty: 'No memories yet',
      emptyHelp:
        'Finish a post and press 기억으로 저장 in ③ to extract some, or write one here by hand.',
      example: 'For example: cannot handle spicy food',
    },
  },
} as const satisfies I18nFragment
