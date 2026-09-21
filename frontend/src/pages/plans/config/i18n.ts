import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `plans` namespace (ARCH-16). */
export const i18n = {
  namespace: 'plans',
  ko: {
    compare: {
      title: '플랜',
      headline: '기록은 더 많이.',
      headlineAccent: '가능성은 더 넓게.',
      allModels: '모든 AI 모델 이용',
      monthlyRefill: '매달 새로운 크레딧',
      editAllowance: '수정까지 고려한 예상 편수',
      closing: '당신의 이야기에 맞는 플랜으로, 다음 이야기를 시작하세요.',
      description: '플랜마다 매달 받는 크레딧이 달라요. 쓸 수 있는 모델은 모든 플랜이 같아요.',
      nav: '플랜',
      monthlyCredits: '매달 {{credits}} 크레딧',
      recommended: '가장 합리적',
      priceUsd: '${{usd}}',
      perMonth: '/ 월',
      priceFree: '무료',
      current: '지금 쓰는 플랜',
      select: '구독하기',
      upgrade: '업그레이드',
      nextBilling: '다음 결제일부터',
      cancelFromBilling: '구독 해지는 결제 관리에서',
      blockedHeading: '크레딧이 부족해요',
      blockedBody:
        'AI 작업을 시작하려면 크레딧이 필요해요. 글을 쓰고 고치고 내보내는 건 그대로 할 수 있어요.',
      buyCredits: '크레딧 구매',
    },
  },
  en: {
    compare: {
      title: 'Plans',
      headline: 'More stories.',
      headlineAccent: 'More possibilities.',
      allModels: 'Every AI model',
      monthlyRefill: 'Fresh credits every month',
      editAllowance: 'Edits included in estimates',
      closing: 'Your next story starts with a plan that fits.',
      description:
        'Plans differ in the credits they grant each month. Every plan can run every model.',
      nav: 'Plans',
      monthlyCredits: '{{credits}} credits a month',
      recommended: 'Best value',
      priceUsd: '${{usd}}',
      perMonth: '/ month',
      priceFree: 'Free',
      current: 'Your current plan',
      select: 'Subscribe',
      upgrade: 'Upgrade',
      nextBilling: 'From next billing date',
      cancelFromBilling: 'Cancel from Billing',
      blockedHeading: 'Out of credits',
      blockedBody:
        'Starting AI work needs credits. Writing, editing and exporting keep working as they are.',
      buyCredits: 'Buy credits',
    },
  },
} as const satisfies I18nFragment
