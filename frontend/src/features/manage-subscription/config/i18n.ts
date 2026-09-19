import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `billing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'billing',
  ko: {
    change: {
      scheduleTitle: '구독 변경을 예약할까요?',
      scheduleConfirm: '변경 예약',
      scheduleDescription: '{{date}}부터 {{plan}} · {{term}}으로 바뀝니다.',
      noChargeNow: '지금은 결제되지 않습니다.',
      quoteFailed: '변경 적용일을 불러오지 못했습니다.',
      scheduled: '{{date}}부터 {{plan}} · {{term}}으로 변경 예정',
      cancelScheduled: '예약 취소',
      switchTerm: '{{term}}으로 바꾸기',
      cancelSubscription: '구독 해지',
      cancelTitle: '구독을 해지할까요?',
      cancelDescription:
        '{{date}}까지 현재 플랜과 남은 크레딧을 그대로 쓸 수 있습니다. 이후 무료 플랜으로 바뀌고 자동 결제되지 않습니다.',
      cancelledForDate: '{{date}}에 구독이 끝납니다. 그 전까지 언제든 다시 시작할 수 있습니다.',
      resume: '해지 취소',
    },
  },
  en: {
    change: {
      scheduleTitle: 'Schedule this subscription change?',
      scheduleConfirm: 'Schedule change',
      scheduleDescription: 'Your subscription changes to {{plan}} · {{term}} on {{date}}.',
      noChargeNow: 'Nothing is charged now.',
      quoteFailed: 'The effective date could not be loaded.',
      scheduled: 'Changes to {{plan}} · {{term}} on {{date}}',
      cancelScheduled: 'Cancel change',
      switchTerm: 'Switch to {{term}}',
      cancelSubscription: 'Cancel subscription',
      cancelTitle: 'Cancel your subscription?',
      cancelDescription:
        'Your current plan and remaining credits stay available through {{date}}. Your account then moves to Free and will not be charged again.',
      cancelledForDate:
        'Your subscription ends on {{date}}. You can resume it any time before then.',
      resume: 'Resume',
    },
  },
} as const satisfies I18nFragment
