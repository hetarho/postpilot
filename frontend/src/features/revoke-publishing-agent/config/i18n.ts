import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `publishing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'publishing',
  ko: {
    revoke: {
      action: '연결 해제',
      failed: '연결을 해제하지 못했어요.',
      title: 'Mac 연결을 해제할까요?',
      description:
        '{{label}}의 발행 토큰이 즉시 무효화됩니다. 네이버 로그인과 브라우저 프로필은 Mac에서 직접 지우기 전까지 남아 있습니다.',
    },
  },
  en: {
    revoke: {
      action: 'Disconnect',
      failed: 'Could not disconnect the Mac.',
      title: 'Disconnect this Mac?',
      description:
        "{{label}}'s publishing token will be invalidated immediately. The Naver login and browser profile remain on the Mac until you remove them there.",
    },
  },
} as const satisfies I18nFragment
