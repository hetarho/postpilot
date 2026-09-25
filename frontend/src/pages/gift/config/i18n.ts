import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `plans` namespace (ARCH-16). */
export const gift = {
  namespace: 'plans',
  ko: {
    gift: {
      loading: '이용권을 확인하는 중…',
      credits: '{{credits}} 크레딧',
      validity: '받은 날부터 {{days}}일 동안 사용',
      defaultMessage: 'Postpilot 이용권이 도착했어요',
      linkExpires: '{{at, instant}}까지 받을 수 있어요',
      redeemed: '이미 받은 이용권이에요',
      expired: '받을 수 있는 기간이 지난 이용권이에요',
      revoked: '취소된 이용권이에요',
      notFound: '이용권을 찾을 수 없어요',
      unavailableBody: '보내 준 분께 새 링크를 요청해 주세요.',
      home: 'Postpilot으로 가기',
    },
  },
  en: {
    gift: {
      loading: 'Checking the voucher…',
      credits: '{{credits}} credits',
      validity: 'Use within {{days}} days of redeeming',
      defaultMessage: 'A Postpilot voucher for you',
      linkExpires: 'Redeem by {{at, instant}}',
      redeemed: 'This voucher has already been redeemed',
      expired: 'This voucher link has expired',
      revoked: 'This voucher has been cancelled',
      notFound: 'This voucher could not be found',
      unavailableBody: 'Ask the person who sent it for a new link.',
      home: 'Go to Postpilot',
    },
  },
} as const satisfies I18nFragment
