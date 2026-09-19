import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `publishing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'publishing',
  ko: {
    retry: {
      pending: '다시 요청하는 중…',
      action: '로그인 복구 후 다시 시도',
      failed: '같은 발행 작업을 다시 시작하지 못했어요. Mac 연결과 카테고리를 확인해 주세요.',
    },
  },
  en: {
    retry: {
      pending: 'Requesting again…',
      action: 'Retry after restoring login',
      failed: 'Could not restart the same publishing job. Check the Mac connection and category.',
    },
  },
} as const satisfies I18nFragment
