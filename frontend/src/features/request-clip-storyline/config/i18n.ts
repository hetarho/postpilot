import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    storylineRequest: {
      label: '스토리라인 수정 요청',
      send: '스토리라인 수정 요청 보내기',
      approve: '최대 {{amount, number}} 크레딧 · 승인하고 스토리라인 고치기',
      running: '스토리라인을 고치는 중이에요',
      count: '{{used}} / {{max}}자',
    },
  },
  en: {
    storylineRequest: {
      label: 'Ask to change the storyline',
      send: 'Send the storyline request',
      approve: 'Up to {{amount, number}} credits · approve and change the storyline',
      running: 'Changing the storyline',
      count: '{{used}} / {{max}} characters',
    },
  },
} as const satisfies I18nFragment
