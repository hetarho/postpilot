import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `billing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'billing',
  ko: {
    purchases: {
      heading: '크레딧 구매',
      empty: '아직 구매한 크레딧이 없습니다.',
      amountLabel: '구매 금액',
      pack: '{{credits}} 크레딧 · {{krw}}원',
      quote: '{{credits}} 크레딧 · {{krw}}원',
      paidRequired: '크레딧 구매는 활성 유료 구독에서만 가능해요.',
      quoteFailed: '구매 금액을 불러오지 못했습니다.',
      paymentRequired: '크레딧을 구매하려면 먼저 카드를 등록해 주세요.',
      buy: '크레딧 구매',
      purchaseTitle: '크레딧을 구매할까요?',
      purchaseDescription:
        '{{credits}} 크레딧에 {{krw}}원이 결제됩니다. 구매 크레딧은 만료되지 않습니다.',
      consumption:
        '구매 크레딧은 만료되는 크레딧과 만료되지 않는 프로모션 크레딧을 사용한 뒤 마지막으로 차감됩니다.',
      row: '{{credits}} 크레딧 · {{krw}}원',
      refund: '환불',
      refundTitle: '크레딧 구매를 환불할까요?',
      refundDescription:
        '구매 후 7일 안에 구매한 크레딧을 하나도 사용하지 않은 경우에만 전액 환불됩니다.',
      refunded: '{{date}} 환불 완료',
      success: {
        purchase: '{{credits}} 크레딧을 구매했습니다.',
        refund: '{{credits}} 크레딧 구매를 환불했습니다.',
      },
    },
  },
  en: {
    purchases: {
      heading: 'Credit purchases',
      empty: 'There are no credit purchases yet.',
      amountLabel: 'Purchase amount',
      pack: '{{credits}} credits · ₩{{krw}}',
      quote: '{{credits}} credits · ₩{{krw}}',
      paidRequired: 'An active paid subscription is required to buy credits.',
      quoteFailed: 'The purchase amount could not be loaded.',
      paymentRequired: 'Register a card before purchasing credits.',
      buy: 'Buy credits',
      purchaseTitle: 'Buy these credits?',
      purchaseDescription:
        '₩{{krw}} will be charged for {{credits}} credits. Purchased credits never expire.',
      consumption:
        'Purchased credits are spent after expiring credits and non-expiring promotional credits.',
      row: '{{credits}} credits · ₩{{krw}}',
      refund: 'Refund',
      refundTitle: 'Refund this credit purchase?',
      refundDescription:
        'A full refund is available within seven days only if none of these purchased credits were used.',
      refunded: 'Refunded {{date}}',
      success: {
        purchase: 'Purchased {{credits}} credits.',
        refund: 'Refunded the purchase of {{credits}} credits.',
      },
    },
  },
} as const satisfies I18nFragment
