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
      inUse: '새 글에 쓰는 말투',
      notDefault: '기본으로 정하면 새 글이 이 말투로 써져요',
      habits: '자주 쓰는 말',
      others: '다른 말투',
      mine: '내 말투',
      noInUse: '아직 새 글에 쓰는 말투가 없어요',
      noInUseHelp: '말투를 완성해 기본으로 정하면, 여기에 그 말투의 문장이 그대로 보여요.',
      readyToMake: '글은 충분해요 · 말투 만들기만 남았어요',
      new: '새 말투',
      methods: {
        learn: { label: '내 글로 배우기', description: '직접 쓴 글이나 답변을 읽고 배워요' },
        ai: {
          label: 'AI와 대화로 만들기',
          description: '원하는 느낌을 말하면 AI가 말투를 만들어요',
        },
      },
    },
    loadFailed: '말투 목록을 불러오지 못했어요.',
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
      inUse: 'The style new posts are written in',
      notDefault: 'Make it the default and new posts use it',
      habits: 'Phrases you use',
      others: 'Other styles',
      mine: 'Your styles',
      noInUse: 'No style is used for new posts yet',
      noInUseHelp: 'Finish a style and make it the default; its own sentences show up here.',
      readyToMake: 'Enough posts · only making the style is left',
      new: 'New style',
      methods: {
        learn: { label: 'Learn from my posts', description: 'Reads posts or answers you wrote' },
        ai: {
          label: 'Make it by talking to AI',
          description: 'Describe the feel and the AI makes the style',
        },
      },
    },
    loadFailed: 'Could not load the voice list.',
  },
} as const satisfies I18nFragment
