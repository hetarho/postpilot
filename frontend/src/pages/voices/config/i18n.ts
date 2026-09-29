import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    page: {
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
  },
  en: {
    page: {
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
  },
} as const satisfies I18nFragment
