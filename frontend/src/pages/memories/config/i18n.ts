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
      bands: {
        always: {
          title: '모든 글에 떠올려요',
          lead: '글을 쓸 때마다 AI가 먼저 떠올리는, 나에 대한 사실이에요.',
        },
        tagged: {
          title: '태그가 맞을 때만 떠올려요',
          lead: '글의 메모·사진·답변에 태그 단어가 나올 때만 꺼내 써요.',
        },
      },
      search: '기억 찾기',
      noMatch: '“{{q}}”에 맞는 기억이 없어요',
      addKind: '{{kind}} 추가',
      kindEmpty: '아직 없어요',
      trigger: '이 단어가 나오면',
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
      bands: {
        always: {
          title: 'Brought into every post',
          lead: 'Facts about you the AI recalls whenever it writes.',
        },
        tagged: {
          title: 'Only when a tag matches',
          lead: 'Used only when a post’s memo, photos or answers mention one of its tags.',
        },
      },
      search: 'Find a memory',
      noMatch: 'No memory matches “{{q}}”',
      addKind: 'Add {{kind}}',
      kindEmpty: 'None yet',
      trigger: 'When a post says',
    },
  },
} as const satisfies I18nFragment
