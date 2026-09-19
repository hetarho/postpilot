import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `publishing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'publishing',
  ko: {
    lastSeen: '마지막 확인',
    agents: {
      title: '발행 Mac',
      description: '계정마다 별도의 Mac 토큰과 전용 브라우저 프로필을 사용합니다.',
      list: '연결 목록',
      loadFailed: '연결 목록을 불러오지 못했어요.',
      empty: '아직 연결한 Mac이 없어요.',
      naverPending: '네이버 확인 대기',
      revoked: '해제됨',
      ready: '준비됨',
      setup: '설정 필요',
      retryTitle: '로그인 복구 후 다시 시도',
      retryDescription:
        'Mac의 같은 전용 브라우저에서 로그인·CAPTCHA·2단계 인증을 해결한 뒤, 고정해 둔 발행 작업을 그대로 다시 시작합니다. 원본 글을 삭제했어도 이 목록에서 재개할 수 있어요.',
      retryLoading: '복구 대기 작업을 불러오는 중…',
      retryLoadFailed: '복구 대기 작업을 불러오지 못했어요.',
      retryEmpty: '복구 후 다시 시도할 작업이 없어요.',
      loginCheck: 'Mac의 전용 네이버 로그인을 확인해 주세요.',
    },
  },
  en: {
    lastSeen: 'Last seen',
    agents: {
      title: 'Publishing Macs',
      description: 'Each account uses a separate Mac token and dedicated browser profile.',
      list: 'Connections',
      loadFailed: 'Could not load the connections.',
      empty: 'No Macs are connected yet.',
      naverPending: 'Waiting for Naver verification',
      revoked: 'Revoked',
      ready: 'Ready',
      setup: 'Setup needed',
      retryTitle: 'Retry after restoring login',
      retryDescription:
        'Resolve login, CAPTCHA, or two-factor authentication in the same dedicated browser on the Mac, then resume the retained publishing job. You can resume here even if the original post was deleted.',
      retryLoading: 'Loading retained jobs…',
      retryLoadFailed: 'Could not load retained jobs.',
      retryEmpty: 'There are no jobs waiting to retry after login recovery.',
      loginCheck: 'Check the dedicated Naver login on the Mac.',
    },
  },
} as const satisfies I18nFragment
