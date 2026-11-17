import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `memories` namespace (ARCH-16). */
export const i18n = {
  namespace: 'memories',
  ko: {
    title: '기억',
    loadFailed: '기억을 불러오지 못했어요.',
    page: {
      statement: 'AI가 알고 있는 내 이야기예요. 글에서 ‘기억 사용’을 켜면 여기서 골라 써요.',
      always: {
        title: '늘 떠올리는 기억',
        how: '취향 · 설정 — 기억을 쓰는 모든 글에서 함께 봐요',
        add: '늘 떠올릴 기억',
        example: '예: 매운 음식을 잘 못 먹는다',
      },
      keyword: {
        title: '단어가 나오면 꺼내는 기억',
        how: '장소 · 인물 · 이력 — 글에 이 단어가 나올 때만 꺼내요',
        add: '단어로 꺼낼 기억',
        example: '예: ‘연남동’ — 주말마다 들르는 동네, 단골 카페가 두 곳 있다',
      },
      none: '아직 없어요',
      trigger: '이 단어가 나오면',
      noTrigger: '단어가 없어 이 기억은 꺼내지 않아요',
      fromPosts: '글 {{count}}편에서 저장',
      fromPosts_one: '글 {{count}}편에서 저장',
      fromPosts_other: '글 {{count}}편에서 저장',
      byHand: '직접 적음',
      noTags: '태그 없음',
    },
  },
  en: {
    title: 'Memories',
    loadFailed: 'Could not load your memories.',
    page: {
      statement:
        'What the AI knows about you. Turn on ‘use memories’ in a post and it picks from here.',
      always: {
        title: 'Always in mind',
        how: 'Preferences · persona — considered for every post that uses memories',
        add: 'Something always true',
        example: 'e.g. I can’t handle spicy food',
      },
      keyword: {
        title: 'Recalled by a word',
        how: 'Places · people · history — recalled only when the post mentions the word',
        add: 'Something recalled by a word',
        example: 'e.g. ‘Yeonnam’ — my weekend neighbourhood, two regular cafés',
      },
      none: 'Nothing yet',
      trigger: 'When the post mentions',
      noTrigger: 'No words, so this is never recalled',
      fromPosts: 'Saved from {{count}} posts',
      fromPosts_one: 'Saved from {{count}} post',
      fromPosts_other: 'Saved from {{count}} posts',
      byHand: 'Written by hand',
      noTags: 'No tags',
    },
  },
} as const satisfies I18nFragment
