import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `plans` namespace (ARCH-16). */
export const i18n = {
  namespace: 'plans',
  ko: {
    revokeVoucher: {
      action: '발급 취소',
      title: '이용권을 취소할까요?',
      unredeemed: '이 링크로는 더 이상 받을 수 없게 돼요.',
      redeemed:
        '받은 계정에 남은 {{credits}} 크레딧을 더 이상 쓸 수 없게 돼요. 이미 쓴 크레딧은 그대로예요.',
      confirm: '발급 취소',
    },
  },
  en: {
    revokeVoucher: {
      action: 'Revoke',
      title: 'Revoke this voucher?',
      unredeemed: 'The link can no longer be redeemed.',
      redeemed:
        'The {{credits}} credits left on the redeeming account stop working. Credits already spent stay spent.',
      confirm: 'Revoke voucher',
    },
  },
} as const satisfies I18nFragment
