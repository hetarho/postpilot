import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `plans` namespace (ARCH-16). */
export const i18n = {
  namespace: 'plans',
  ko: {
    redeemVoucher: {
      redeem: '받기',
      signIn: '로그인하고 받기',
      signUp: '가입하고 받기',
      redeemed: '{{credits}} 크레딧을 받았어요. {{at, instant}}까지 쓸 수 있어요.',
      start: '시작하기',
    },
  },
  en: {
    redeemVoucher: {
      redeem: 'Redeem',
      signIn: 'Log in to redeem',
      signUp: 'Sign up to redeem',
      redeemed: '{{credits}} credits added. Use them until {{at, instant}}.',
      start: 'Get started',
    },
  },
} as const satisfies I18nFragment
