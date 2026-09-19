import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `publishing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'publishing',
  ko: {
    // The agent cannot carry a clip yet (VIDEO-16).
    blocked: {
      videoBlock: '영상이 들어간 글은 아직 발행할 수 없어요. 내보내기에서 직접 붙여 주세요.',
    },
    form: {
      unavailableAgent:
        '이 작업에 연결된 Mac을 현재 사용할 수 없어 다른 Mac으로 바꾸어 재시도하지 않았어요. Mac 연결을 다시 활성화하거나 이 작업을 취소해 주세요.',
      cancel: '발행 취소',
      cancelFailed: '발행을 취소하지 못했어요.',
      finalizeFirst: '현재 내용을 먼저 확정해야 정확히 이 버전을 발행할 수 있어요.',
      offline:
        'Mac이 지금 응답하지 않아도 요청은 서버에 보관되고, 에이전트가 켜지면 자동으로 시작됩니다.',
      agent: 'Mac 연결',
      category: '카테고리',
      visibility: '공개 설정',
      changedFinalize: '방금 수정한 내용을 다시 확정한 뒤 발행해 주세요.',
      saveFailed: '수정 내용을 저장하지 못해 발행을 시작하지 않았어요.',
      retry: '안전하게 다시 시도',
      publish: '네이버에 발행',
      confirmTitle: '네이버에 최종 발행할까요?',
      confirmDescription:
        '{{account}} 블로그의 {{category}} 카테고리에 올립니다. Mac 에이전트가 사진과 글을 입력한 뒤 네이버의 최종 발행 버튼까지 누르며, 그 순간에는 추가 확인을 요청하지 않습니다.',
    },
  },
  en: {
    // The agent cannot carry a clip yet (VIDEO-16).
    blocked: {
      videoBlock:
        'A post with a video cannot be published yet. Paste it yourself from the export tab.',
    },
    form: {
      unavailableAgent:
        'The Mac assigned to this job is unavailable, so Postpilot did not retry it through a different Mac. Reactivate the connection or cancel this job.',
      cancel: 'Cancel publishing',
      cancelFailed: 'Could not cancel publishing.',
      finalizeFirst: 'Finalize the current content first to publish this exact version.',
      offline:
        'Even if the Mac is not responding now, the request stays on the server and starts automatically when the agent comes online.',
      agent: 'Mac connection',
      category: 'Category',
      visibility: 'Visibility',
      changedFinalize: 'Finalize the changes you just made before publishing.',
      saveFailed: 'Publishing did not start because the changes could not be saved.',
      retry: 'Retry safely',
      publish: 'Publish to Naver',
      confirmTitle: 'Publish to Naver now?',
      confirmDescription:
        "This will publish to the {{category}} category on the {{account}} blog. The Mac agent enters the photos and content, then presses Naver's final publish button without asking for another confirmation.",
    },
  },
} as const satisfies I18nFragment
