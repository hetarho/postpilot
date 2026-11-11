import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'creation',
  ko: {
    settings: {
      title: '나',
      credits: '남은 크레딧',
      charge: '충전하기',
      planGroup: '이용',
      accountGroup: '계정',
      screen: '화면',
      account: '계정',
      plans: '요금제',
      billing: '결제와 구독',
      admin: '관리자',
    },
  },
  en: {
    settings: {
      title: 'Me',
      credits: 'Credits left',
      charge: 'Top up',
      planGroup: 'Plan',
      accountGroup: 'Account',
      screen: 'Display',
      account: 'Account',
      plans: 'Plans',
      billing: 'Billing and subscription',
      admin: 'Administration',
    },
  },
} as const satisfies I18nFragment
