import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    preview: {
      speechMissing: '더빙 {{segment}}: 음성이 아직 없어요.',
      speechStale: '더빙 {{segment}}: 바뀐 대본이나 목소리의 음성이 필요해요.',
      speechConflict: '더빙 {{segment}}: 음성을 자르지 않고 들으려면 시간 배치를 수정해 주세요.',
      previousSpeech: '이전 음성은 더빙 항목에서 따로 들을 수 있어요.',
      speechLoading: '재생할 소리를 준비하고 있어요.',
      audioFailed: '요청한 소리를 재생하지 못했어요. 현재 더빙이나 원본 소리를 확인해 주세요.',
      playbackGesture: '재생 버튼을 다시 눌러 소리를 시작해 주세요.',
      speechErrorMemory: '더빙 재생에 필요한 메모리가 부족해요.',
      speechErrorDecode: '더빙 {{segment}}을 이 브라우저에서 해독하지 못했어요.',
      speechErrorGesture: '재생 버튼을 다시 눌러 더빙 소리를 시작해 주세요.',
      speechErrorPending: '현재 더빙 음성을 먼저 준비해 주세요.',
      speechErrorUnavailable:
        '더빙 {{segment}}의 음성을 불러오지 못했어요. 새로고침 후 다시 재생해 주세요.',

      aboutLabel: '이 미리보기에 대해',
      invalidTimeline: '재생 가능한 컷 구간을 입력해 주세요. 글과 시간은 계속 수정할 수 있어요.',
      title: '편집 중인 영상',
      play: '재생',
      pause: '일시 정지',
      replay: '처음부터 재생',
      refresh: '미리보기 새로고침',
      flowView: '흐름 보기',
      videoView: '영상 보기',
      cutNumber: '컷 {{n}}',
      flowParity: '멈춘 장면으로 흐름만 보여줘요. 실제 렌더와 다를 수 있어요.',
      originalAudio: '미리보기 소리 듣기',
      outputTime: '완성 영상 기준 시간',
      loadingMedia: '원본 영상을 불러오고 있어요.',
      codec: '이 브라우저에서는 원본 형식을 재생할 수 없어요. 글과 시간은 계속 수정할 수 있어요.',
      expired: '원본 보관 시간이 지났어요. 같은 원본을 다시 선택하면 미리 볼 수 있어요.',
      missing: '원본을 찾을 수 없어요. 같은 원본을 다시 선택해 주세요.',
      mediaFailed: '원본을 재생하지 못했어요. 글과 시간은 계속 수정할 수 있어요.',
      preparationFailed:
        '편집 내용의 미리보기를 준비하지 못했어요. 내용과 표시 시간을 확인해 주세요.',
      updating: '변경한 문구의 미리보기를 갱신하고 있어요.',
      currentDraft: '현재 편집 내용의 미리보기예요.',
      retry: '미리보기 다시 준비',
      parity:
        '브라우저 재생 시간에는 오차가 있을 수 있어요. 원본에 맞춘 글자 대비와 최종 음량은 영상 만들기 후 확인해 주세요.',
      frameApproximate:
        '브라우저 재생 위치는 근사치예요. 정확한 프레임은 완성 영상에서 확인해 주세요.',
      renderedRevision: '이전에 만든 영상 비교 · 수정본 {{revision}}',
    },
  },
  en: {
    preview: {
      speechMissing: 'Speech {{segment}}: audio has not been generated.',
      speechStale: 'Speech {{segment}} needs audio for the changed script or voice.',
      speechConflict: 'Speech {{segment}} needs timing correction to play without truncation.',
      previousSpeech: 'Previous audio is available separately in the speech item.',
      speechLoading: 'Preparing playback audio.',
      audioFailed:
        'The requested audio could not play. Check the current narration or original audio.',
      playbackGesture: 'Press play again to start audio.',
      speechErrorMemory: 'Not enough memory to play the narration.',
      speechErrorDecode: 'This browser could not decode speech {{segment}}.',
      speechErrorGesture: 'Press play again to start narration audio.',
      speechErrorPending: 'Prepare current narration audio first.',
      speechErrorUnavailable: 'Could not load speech {{segment}}. Refresh and try playback again.',

      aboutLabel: 'About this preview',
      invalidTimeline:
        'Enter valid cut ranges to preview footage. Text and timing remain editable.',
      title: 'Current draft preview',
      play: 'Play',
      pause: 'Pause',
      replay: 'Play from the start',
      refresh: 'Refresh preview',
      flowView: 'Flow view',
      videoView: 'Video view',
      cutNumber: 'Cut {{n}}',
      flowParity: 'Still frames show the flow only; the render may differ.',
      originalAudio: 'Enable preview audio',
      outputTime: 'Output timeline',
      loadingMedia: 'Loading the original video.',
      codec: 'This browser cannot play the original format. Text and timing remain editable.',
      expired: 'The original has expired. Select the matching original to preview it again.',
      missing: 'The original is missing. Select the matching original again.',
      mediaFailed: 'The original could not play. Text and timing remain editable.',
      preparationFailed:
        'Could not prepare this draft preview. Check its text and visibility intervals.',
      updating: 'Updating the preview for your edited text.',
      currentDraft: 'Previewing the current draft.',
      retry: 'Retry preview preparation',
      parity:
        'Browser timing is approximate. Check source-dependent text contrast and final audio normalization in the completed video.',
      frameApproximate:
        'Browser playback timing is approximate. Check exact frames in the completed video.',
      renderedRevision: 'Compare previous render · revision {{revision}}',
    },
  },
} as const satisfies I18nFragment
