import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `billing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'billing',
  ko: {
    checkout: {
      title: '구독 시작',
      description: '플랜과 결제 주기를 확인한 뒤 구독을 시작합니다.',
      invalid: '구독할 유료 플랜을 다시 선택해 주세요.',
      loadFailed: '구독 가격을 불러오지 못했습니다.',
      monthlyGrant: '매달 {{credits}} 크레딧',
      termHeading: '결제 주기',
      term: { monthly: '월간', annual: '연간' },
      annualValue: '12개월에 10개월 요금',
      priceHeading: '오늘의 예상 결제 금액',
      price: '${{usd}} · {{krw}}원',
      rate: '{{date}} 기준 환율 · 1달러당 {{rate}}원',
      moving: '원화 금액은 매일 바뀌는 기준 환율을 따르므로 실제 결제일에는 달라질 수 있습니다.',
      paymentRequired: '구독하려면 먼저 카드를 등록해 주세요.',
      submit: '결제하고 구독하기',
      upgradeSubmit: '지금 결제하고 업그레이드',
      chargedNow: '지금 결제되는 업그레이드 금액입니다.',
      back: '플랜으로 돌아가기',
    },
  },
  en: {
    checkout: {
      title: 'Start subscription',
      description: 'Confirm your plan and billing term before subscribing.',
      invalid: 'Choose a paid plan to subscribe to.',
      loadFailed: 'The subscription price could not be loaded.',
      monthlyGrant: '{{credits}} credits each month',
      termHeading: 'Billing term',
      term: { monthly: 'Monthly', annual: 'Annual' },
      annualValue: '12 months for the price of 10',
      priceHeading: "Today's estimated charge",
      price: '${{usd}} · ₩{{krw}}',
      rate: '{{date}} base rate · ₩{{rate}} per USD',
      moving: 'The won amount follows the daily base rate and may differ on the charge day.',
      paymentRequired: 'Register a card before subscribing.',
      submit: 'Pay and subscribe',
      upgradeSubmit: 'Pay now and upgrade',
      chargedNow: 'This upgrade amount is charged now.',
      back: 'Back to plans',
    },
  },
} as const satisfies I18nFragment
