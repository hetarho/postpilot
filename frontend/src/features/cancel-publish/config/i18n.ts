import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `publishing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'publishing',
  ko: {
    cancelRetained: {
      action: '복구 작업 취소',
      failed: '복구 작업을 취소하지 못했어요.',
      title: '고정해 둔 발행 작업을 취소할까요?',
      confirm: '작업 취소',
      description:
        '{{postSlug}}의 고정된 글과 임시 사진을 삭제합니다. 이 작업은 다시 시도할 수 없습니다.',
    },
  },
  en: {
    cancelRetained: {
      action: 'Cancel recovery job',
      failed: 'Could not cancel the recovery job.',
      title: 'Cancel this retained publishing job?',
      confirm: 'Cancel job',
      description:
        'The retained content and temporary photos for {{postSlug}} will be deleted. This job cannot be retried afterward.',
    },
  },
} as const satisfies I18nFragment
