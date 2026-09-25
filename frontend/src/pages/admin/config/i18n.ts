import type { I18nFragment } from '@/shared/lib'

/** The vouchers tab's share of the `plans` namespace (ARCH-16). */
export const vouchers = {
  namespace: 'plans',
  ko: {
    adminVouchers: {
      description: '계좌이체로 판매하거나 선물로 주는 이용권을 발급하고 관리합니다.',
      heading: '발급한 이용권',
      loading: '이용권을 불러오는 중…',
      loadFailed: '이용권을 불러오지 못했어요.',
      empty: '아직 발급한 이용권이 없어요.',
      summary: '{{credits}} 크레딧 · {{days}}일',
      issued: '{{date}} 발급',
      sold: '판매 {{amount}}원 · {{payer}}',
      given: '선물',
      state: {
        redeemable: '받기 전',
        redeemed: '받음',
        expired: '링크 만료',
        revoked: '취소됨',
        unknown: '알 수 없음',
      },
      redeemedBy: '{{account}} · {{date}} 받음',
      remaining: '남은 크레딧 {{credits}} · {{date}}까지',
    },
  },
  en: {
    adminVouchers: {
      description: 'Issue and manage vouchers sold by bank transfer or given as gifts.',
      heading: 'Issued vouchers',
      loading: 'Loading vouchers…',
      loadFailed: 'Could not load vouchers.',
      empty: 'No vouchers issued yet.',
      summary: '{{credits}} credits · {{days}} days',
      issued: 'Issued {{date}}',
      sold: 'Sold {{amount}} KRW · {{payer}}',
      given: 'Gift',
      state: {
        redeemable: 'Not redeemed',
        redeemed: 'Redeemed',
        expired: 'Link expired',
        revoked: 'Revoked',
        unknown: 'Unknown',
      },
      redeemedBy: 'Redeemed by {{account}} · {{date}}',
      remaining: '{{credits}} credits left · until {{date}}',
    },
  },
} as const satisfies I18nFragment
