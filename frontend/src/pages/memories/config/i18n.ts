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
      lead: 'AI가 기억하는 것',
      saved: '저장된 기억',
      order: '최근에 쓰인 기억이 먼저 옵니다. 글에 들어가는 순서도 같아요.',
      empty: '아직 기억이 없어요',
      emptyHelp:
        '글을 완성한 뒤 ③에서 기억으로 저장을 눌러 뽑아내거나, 여기서 직접 적을 수 있어요.',
      example: '예: 매운 음식을 잘 못 먹는다',
      noTags: '태그 없음',
    },
    notebook: {
      tags: '태그로 찾기',
      all: '전체 {{count}}',
      add: '추가',
      addKind: '{{kind}} 기억 추가',
      blank: '아직 빈 페이지',
      blankFirst: 'AI에게 알려 줄 수 있는 것',
      firstTitle: '아직 기억이 없어요',
      firstLine: '나에 대한 사실을 적어 두면, AI가 글마다 알맞은 것을 꺼내 써요.',
      empty: {
        preference: '좋아하는 것, 못 먹는 것, 자주 가는 곳',
        persona: '나는 어떤 사람인지, 무슨 일을 하는지',
        place: '내 가게, 동네, 자주 쓰는 장소',
        person: '글에 자주 나오는 사람',
        history: '전에 해 본 것, 지나온 일',
      },
    },
  },
  en: {
    title: 'Memories',
    loadFailed: 'Could not load your memories.',
    page: {
      description:
        'Facts about you that no single post carries. Only a post with 기억 사용 turned on in ① is written with them.',
      lead: 'What the AI remembers',
      saved: 'Saved memories',
      order: 'Most recently used first — the same order a post is given them in.',
      empty: 'No memories yet',
      emptyHelp:
        'Finish a post and press 기억으로 저장 in ③ to extract some, or write one here by hand.',
      example: 'For example: cannot handle spicy food',
      noTags: 'No tags',
    },
    notebook: {
      tags: 'Find by tag',
      all: 'All {{count}}',
      add: 'Add',
      addKind: 'Add a {{kind}} memory',
      blank: 'Blank pages',
      blankFirst: 'What you can tell the AI',
      firstTitle: 'No memories yet',
      firstLine: 'Write down facts about you, and the AI brings the right ones into each post.',
      empty: {
        preference: 'What you like, cannot eat, keep going back to',
        persona: 'Who you are, what you do',
        place: 'Your shop, your area, places you use',
        person: 'People who often appear in your posts',
        history: 'Things you have done before',
      },
    },
  },
} as const satisfies I18nFragment
