import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `billing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'billing',
  ko: {
    checkout: {
      title: '구독 시작',
      description: '플랜과 결제 주기를 확인한 뒤 구독을 시작합니다.',
      serverExports: '서버 내보내기 월 {{count}}회',
      invalid: '구독할 유료 플랜을 다시 선택해 주세요.',
      loadFailed: '구독 가격을 불러오지 못했습니다.',
      benefits: '하루 {{daily}} 크레딧 · 매월 보너스 {{bonus}} 크레딧',
      termHeading: '결제 주기',
      term: { monthly: '월간', annual: '연간' },
      annualValue: '12개월에 10개월 요금',
      priceHeading: '결제 금액',
      price: '{{krw}}원',
      fixedPrice: '부가세가 포함된 고정 원화 가격입니다.',
      annualUpfront: '연간 요금 전액을 지금 결제하며, 월 혜택은 매월 갱신됩니다.',
      proration: '남은 구독 기간을 기준으로 계산한 정확한 업그레이드 금액입니다.',
      upgradeTiming:
        '{{at}}부터 상위 모델과 월 혜택을 이용할 수 있고 일일 지급은 기존 다음 지급 시각 {{dailyAt}}부터 늘어납니다.',
      paymentPending: '결제사 결과를 확인 중입니다. 결제가 확인되면 구독이 반영됩니다.',
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
      serverExports: '{{count}} server exports per month',
      invalid: 'Choose a paid plan to subscribe to.',
      loadFailed: 'The subscription price could not be loaded.',
      benefits: '{{daily}} credits daily · {{bonus}} monthly bonus credits',
      termHeading: 'Billing term',
      term: { monthly: 'Monthly', annual: 'Annual' },
      annualValue: '12 months for the price of 10',
      priceHeading: 'Amount payable',
      price: '₩{{krw}}',
      fixedPrice: 'This is a fixed, VAT-inclusive KRW price.',
      annualUpfront: 'The full annual price is charged now; monthly benefits renew each month.',
      proration: 'This exact upgrade charge covers the remaining subscription term.',
      upgradeTiming:
        'Higher model access and monthly benefits begin {{at}}; the daily increase begins at the existing next reset, {{dailyAt}}.',
      paymentPending:
        'Confirming the provider outcome. Your subscription updates after confirmation.',
      paymentRequired: 'Register a card before subscribing.',
      submit: 'Pay and subscribe',
      upgradeSubmit: 'Pay now and upgrade',
      chargedNow: 'This upgrade amount is charged now.',
      back: 'Back to plans',
    },
  },
} as const satisfies I18nFragment
