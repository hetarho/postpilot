import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `publishing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'publishing',
  ko: {
    panel: {
      description:
        '연결한 Mac이 네이버 편집기를 열어 글과 JPEG 사진을 입력하고 최종 발행까지 마칩니다.',
      loading: '발행 상태를 불러오는 중…',
      loadFailed: '발행 상태를 불러오지 못했어요.',
      reload: '발행 상태 다시 불러오기',
      noAgent:
        '발행할 수 있는 Mac 연결이 아직 없어요. 네이버 로그인 정보는 Mac 밖으로 전송되지 않습니다.',
      connectAgent: 'Mac 연결하기',
      published: '발행을 마쳤어요.',
      viewPost: '네이버 글 보기',
      failed: '최종 발행 전에 안전하게 중단했어요.',
      needsAttention:
        'Mac의 전용 브라우저에서 네이버 로그인을 확인한 뒤 아래에서 같은 작업을 다시 시도하세요.',
      outcomeUnknown:
        '최종 발행 버튼이 눌렸을 수 있어요. 중복을 막기 위해 자동 재시도하지 않습니다. 네이버 블로그에서 직접 확인해 주세요.',
    },
  },
  en: {
    panel: {
      description:
        'A connected Mac opens the Naver editor, enters the post and JPEG photos, and completes publishing.',
      loading: 'Loading publishing status…',
      loadFailed: 'Could not load publishing status.',
      reload: 'Reload publishing status',
      noAgent: 'No Mac is ready to publish yet. Your Naver login never leaves the Mac.',
      connectAgent: 'Connect a Mac',
      published: 'Publishing is complete.',
      viewPost: 'View on Naver',
      failed: 'The job stopped safely before final publishing.',
      needsAttention:
        "Check the Naver login in the Mac's dedicated browser, then retry the same job below.",
      outcomeUnknown:
        'The final publish button may have been pressed. To prevent duplicates, the job will not retry automatically. Check Naver Blog directly.',
    },
  },
} as const satisfies I18nFragment
