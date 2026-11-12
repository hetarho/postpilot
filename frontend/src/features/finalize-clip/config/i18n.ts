import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    confirmation: {
      action: '확정하기',
      refusal: {
        finalized: '완료된 영상이라 다시 확정할 수 없어요.',
        busy: '진행 중인 작업이 끝나면 확정할 수 있어요.',
        invalid_plan: '편집 내용의 오류를 고친 뒤 확정해 주세요.',
        narration: '바뀐 대본의 음성을 다시 만든 뒤 확정해 주세요.',
        unavailable: '확정할 수 있는지 확인하지 못했어요. 화면을 새로고침해 주세요.',
      },
    },
    publishedUrl: {
      title: '게시 URL',
      label: { naver_clip: '네이버 클립 주소', instagram_clip: '인스타그램 릴스 주소' },
      placeholder: {
        naver_clip: 'https://clip.naver.com/…',
        instagram_clip: 'https://www.instagram.com/reel/…',
      },
      save: 'URL 저장',
      replace: 'URL 바꾸기',
      saved: '게시 URL을 저장했어요.',
      dialogTitle: '게시 URL을 저장하고 클립을 완료할까요?',
      dialogConfirm: '저장하고 완료',
    },
    finalization: {
      confirm: '완료로 표시',
      dialogTitle: '클립을 완료로 표시할까요?',
      notice:
        '완료로 표시하면 원본을 삭제하고 수정이 끝나요. 완료된 영상은 계속 재생하고 다운로드할 수 있어요.',
      waiting:
        '영상 다듬기에서 확정하기를 누르면 이 단계에서 렌더해요. 다운로드만으로는 확정되지 않아요.',
      goRefine: '영상 다듬기로 가기',
      uncertain:
        '확정 결과를 확인하고 있어요. 서버 상태가 확인될 때까지 수정과 재시도를 잠시 멈춰요.',
      downloadRevision: '렌더 {{revision}} 다운로드',
      refusal: {
        finalized: '이미 완료된 클립이에요.',
        busy: '작업이 끝난 뒤 확정할 수 있어요.',
        unconfirmed: '영상 다듬기에서 확정하기를 누른 뒤 완료로 표시할 수 있어요.',
        missing_render: '렌더하기로 영상을 만든 뒤 완료로 표시해 주세요.',
        stale_render: '렌더하기로 현재 편집안을 출력한 뒤 완료로 표시해 주세요.',
        invalid_plan: '편집 내용의 오류를 수정하고 다시 렌더해 주세요.',
        unavailable: '확정 가능 여부를 확인하지 못했어요. 화면을 새로고침해 주세요.',
      },
    },
  },
  en: {
    confirmation: {
      action: 'Confirm',
      refusal: {
        finalized: 'This clip is complete and cannot be confirmed again.',
        busy: 'You can confirm once the current job finishes.',
        invalid_plan: 'Fix the editing errors before confirming.',
        narration: 'Regenerate the speech for the changed script before confirming.',
        unavailable: 'Could not check whether this clip can be confirmed. Refresh this page.',
      },
    },
    publishedUrl: {
      title: 'Posted URL',
      label: { naver_clip: 'Naver Clip address', instagram_clip: 'Instagram reel address' },
      placeholder: {
        naver_clip: 'https://clip.naver.com/…',
        instagram_clip: 'https://www.instagram.com/reel/…',
      },
      save: 'Save URL',
      replace: 'Replace URL',
      saved: 'Saved the posted URL.',
      dialogTitle: 'Save the posted URL and complete this clip?',
      dialogConfirm: 'Save and complete',
    },
    finalization: {
      confirm: 'Mark complete',
      dialogTitle: 'Mark this clip complete?',
      notice:
        'Marking it complete deletes the originals and ends editing. You can still play and download the completed video.',
      waiting:
        'Confirm the clip in Refine video and it renders here. Downloading alone does not confirm it.',
      goRefine: 'Go to Refine video',
      uncertain:
        'Checking confirmation. Editing and retries stay paused until the server state is known.',
      downloadRevision: 'Download render {{revision}}',
      refusal: {
        finalized: 'This clip is already complete.',
        busy: 'Wait for the current job before confirming.',
        unconfirmed: 'Confirm the clip in Refine before marking it complete.',
        missing_render: 'Use Render to make a video before marking it complete.',
        stale_render: 'Use Render to output the current edits before marking it complete.',
        invalid_plan: 'Fix the editing errors and render again.',
        unavailable: 'Confirmation availability is unknown. Refresh this page.',
      },
    },
  },
} as const satisfies I18nFragment
