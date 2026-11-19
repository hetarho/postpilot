import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `billing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'billing',
  ko: {
    creditsCard: {
      credits: '남은 크레딧',
      charge: '충전하기',
      packs: '크레딧 구매',
      packsLoadFailed: '결제 정보를 불러오지 못했습니다.',
    },
  },
  en: {
    creditsCard: {
      credits: 'Credits left',
      charge: 'Top up',
      packs: 'Credit purchases',
      packsLoadFailed: 'Billing information could not be loaded.',
    },
  },
} as const satisfies I18nFragment
