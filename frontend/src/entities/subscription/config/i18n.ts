import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `billing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'billing',
  ko: {
    title: '결제 관리',
    paymentMethod: {
      heading: '결제 수단',
      empty: '등록된 결제 수단이 없습니다.',
      card: '등록 카드',
      registeredAt: '등록일',
      register: '카드 등록',
      change: '카드 바꾸기',
      remove: '카드 삭제',
      removeTitle: '등록 카드를 삭제할까요?',
      removeDescription: '구독 자동 갱신 중에는 카드를 삭제할 수 없습니다.',
      emailRequired: '카드를 등록하려면 인증된 이메일이 필요합니다.',
      verifyEmail: '이메일 인증하기',
      openFailed: '카드 등록 창을 열지 못했습니다. 다시 시도해 주세요.',
    },
    registration: {
      title: '카드 등록',
      checking: '카드 정보를 안전하게 등록하는 중입니다…',
      invalid: '카드 등록 응답이 올바르지 않습니다.',
      failed: '카드 등록을 완료하지 못했습니다. 다시 시도해 주세요.',
      back: '결제 관리로 돌아가기',
      done: '{{label}} 카드가 등록되었습니다.',
      doneWithBonus: '{{label}} 카드가 등록되었고 {{credits}} 크레딧 보너스가 지급되었습니다.',
    },
  },
  en: {
    title: 'Billing',
    paymentMethod: {
      heading: 'Payment method',
      empty: 'There is no registered payment method.',
      card: 'Registered card',
      registeredAt: 'Registered',
      register: 'Register card',
      change: 'Change card',
      remove: 'Remove card',
      removeTitle: 'Remove the registered card?',
      removeDescription: 'A card cannot be removed while a subscription is set to renew.',
      emailRequired: 'A verified email is required to register a card.',
      verifyEmail: 'Verify email',
      openFailed: 'The card registration window could not be opened. Please try again.',
    },
    registration: {
      title: 'Register card',
      checking: 'Securely registering your card…',
      invalid: 'The card registration response is invalid.',
      failed: 'Card registration could not be completed. Please try again.',
      back: 'Back to billing',
      done: '{{label}} was registered.',
      doneWithBonus: '{{label}} was registered and the {{credits}}-credit bonus was granted.',
    },
  },
} as const satisfies I18nFragment
