import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    finalization: {
      confirm: '확정하기',
      dialogTitle: '클립을 확정할까요?',
      notice:
        '확정하면 원본을 삭제하고 수정이 끝나요. 확정된 영상은 계속 재생하고 다운로드할 수 있어요.',
      waiting: '수정 단계에서 확정하기를 누르면 완성돼요. 다운로드만으로는 확정되지 않아요.',
      goRefine: '수정으로 이동',
      uncertain:
        '확정 결과를 확인하고 있어요. 서버 상태가 확인될 때까지 수정과 재시도를 잠시 멈춰요.',
      downloadRevision: '렌더 {{revision}} 다운로드',
      refusal: {
        finalized: '이미 확정된 클립이에요.',
        busy: '작업이 끝난 뒤 확정할 수 있어요.',
        missing_render: '렌더하기로 영상을 만든 뒤 확정해 주세요.',
        stale_render: '렌더하기로 현재 편집안을 출력한 뒤 확정해 주세요.',
        invalid_plan: '편집 내용의 오류를 수정하고 다시 렌더해 주세요.',
        unavailable: '확정 가능 여부를 확인하지 못했어요. 화면을 새로고침해 주세요.',
      },
    },
  },
  en: {
    finalization: {
      confirm: 'Confirm clip',
      dialogTitle: 'Confirm this clip?',
      notice:
        'Confirmation deletes the originals and ends editing. You can still play and download the confirmed video.',
      waiting:
        'Confirm the clip in the editing step to finish. Downloading alone does not confirm it.',
      goRefine: 'Go to Refine',
      uncertain:
        'Checking confirmation. Editing and retries stay paused until the server state is known.',
      downloadRevision: 'Download render {{revision}}',
      refusal: {
        finalized: 'This clip is already confirmed.',
        busy: 'Wait for the current job before confirming.',
        missing_render: 'Use Render to make a video before confirming.',
        stale_render: 'Use Render to output the current edits before confirming.',
        invalid_plan: 'Fix the editing errors and render again.',
        unavailable: 'Confirmation availability is unknown. Refresh this page.',
      },
    },
  },
} as const satisfies I18nFragment
