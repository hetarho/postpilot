import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `plans` namespace (ARCH-16). */
export const i18n = {
  namespace: 'plans',
  ko: {
    redeemVoucher: {
      redeem: '받기',
      signIn: '로그인하고 받기',
      signUp: '가입하고 받기',
      checking: '구독 상태를 확인하는 중…',
      paidRequired:
        '이용권은 활성 유료 구독 중에 받을 수 있어요. 구독 후 이 링크로 돌아와 받기를 눌러 주세요.',
      plans: '요금제 보기',
      redeemed: '{{credits}} 크레딧을 받았어요. {{at, instant}}까지 쓸 수 있어요.',
      start: '시작하기',
    },
  },
  en: {
    redeemVoucher: {
      redeem: 'Redeem',
      signIn: 'Log in to redeem',
      signUp: 'Sign up to redeem',
      checking: 'Checking your subscription…',
      paidRequired:
        'Redeeming this gift requires an active paid subscription. Subscribe, then return to this link to redeem it.',
      plans: 'View plans',
      redeemed: '{{credits}} credits added. Use them until {{at, instant}}.',
      start: 'Get started',
    },
  },
} as const satisfies I18nFragment
