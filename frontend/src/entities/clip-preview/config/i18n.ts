import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    preview: {
      aboutLabel: '이 미리보기에 대해',
      invalidTimeline: '재생 가능한 컷 구간을 입력해 주세요. 글과 시간은 계속 수정할 수 있어요.',
      title: '편집 중인 영상',
      play: '재생',
      pause: '일시 정지',
      replay: '처음부터 재생',
      refresh: '미리보기 새로고침',
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
      aboutLabel: 'About this preview',
      invalidTimeline:
        'Enter valid cut ranges to preview footage. Text and timing remain editable.',
      title: 'Current draft preview',
      play: 'Play',
      pause: 'Pause',
      replay: 'Play from the start',
      refresh: 'Refresh preview',
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
