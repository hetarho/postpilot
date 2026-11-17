import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    page: {
      lead: '이런 말투로 써요',
      empty: '아직 말투가 없어요. 직접 쓴 글이나 문항 답으로 나만의 말투를 만들어 보세요.',
      making: '만드는 중 {{percent}}%',
      meta: '학습 글 {{count}}편 · {{date}} 분석',
      meta_one: '학습 글 {{count}}편 · {{date}} 분석',
      meta_other: '학습 글 {{count}}편 · {{date}} 분석',
      deleted: '삭제된 말투 {{count}}개',
      deleted_one: '삭제된 말투 {{count}}개',
      deleted_other: '삭제된 말투 {{count}}개',
    },
    loadFailed: '말투 목록을 불러오지 못했어요.',
    specimen: {
      eyebrow: '이 말투로 쓰면',
      empty: '아직 말투가 없어요',
      unmade: '분석하면 이 말투로 쓴 문장이 여기에 보여요.',
      phrases: '자주 쓰는 말',
      open: '자세히 보기',
      continue: '이어서 만들기',
      pick: '말투 고르기',
      createOwn: '내가 쓴 글로 만들기',
      createOwnHelp: '직접 쓴 글과 문항 답에서 배워요',
      createAIHelp: '원하는 느낌을 말하면 AI가 말투를 제안해요',
    },
  },
  en: {
    page: {
      lead: 'How your posts sound',
      empty:
        'No voices yet. Make your own from posts you wrote by hand or from your answers to the prompts.',
      making: 'Being made {{percent}}%',
      meta: '{{count}} writings · analyzed {{date}}',
      meta_one: '{{count}} writing · analyzed {{date}}',
      meta_other: '{{count}} writings · analyzed {{date}}',
      deleted: '{{count}} deleted voices',
      deleted_one: '{{count}} deleted voice',
      deleted_other: '{{count}} deleted voices',
    },
    loadFailed: 'Could not load the voice list.',
    specimen: {
      eyebrow: 'Written in this style',
      empty: 'No styles yet',
      unmade: 'Once analyzed, sentences in this style show here.',
      phrases: 'Phrases it keeps using',
      open: 'Open',
      continue: 'Keep making it',
      pick: 'Choose a style',
      createOwn: 'From my own writing',
      createOwnHelp: 'Learns from posts you wrote and your answers',
      createAIHelp: 'Describe a feel and the AI suggests styles',
    },
  },
} as const satisfies I18nFragment
