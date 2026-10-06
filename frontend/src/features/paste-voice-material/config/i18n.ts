import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    paste: {
      open: '글 붙여넣기',
      title: '글 붙여넣기',
      label: '제목 (선택)',
      labelPlaceholder: '예: 제주 여행기',
      body: '내가 쓴 글',
      bodyPlaceholder: '직접 쓴 글 한 편을 붙여 넣어 주세요',
      count: '{{count}} / {{min}}자',
      count_one: '{{count}} / {{min}}자',
      count_other: '{{count}} / {{min}}자',
      remaining: '{{count}}자 더 필요해요',
      remaining_one: '{{count}}자 더 필요해요',
      remaining_other: '{{count}}자 더 필요해요',
      submit: '추가',
      added: '글을 추가했어요',
      confirmFailed: '글의 저장을 확인하지 못했어요. 입력한 내용은 남아 있어요.',
    },
  },
  en: {
    paste: {
      open: 'Paste a post',
      title: 'Paste a post',
      label: 'Title (optional)',
      labelPlaceholder: 'For example: Jeju travel story',
      body: 'A post I wrote',
      bodyPlaceholder: 'Paste one post you wrote yourself',
      count: '{{count}} / {{min}} characters',
      count_one: '{{count}} / {{min}} character',
      count_other: '{{count}} / {{min}} characters',
      remaining: '{{count}} more characters needed',
      remaining_one: '{{count}} more character needed',
      remaining_other: '{{count}} more characters needed',
      submit: 'Add',
      added: 'Post added',
      confirmFailed: 'Could not confirm the saved material. Your entered text remains.',
    },
  },
} as const satisfies I18nFragment
