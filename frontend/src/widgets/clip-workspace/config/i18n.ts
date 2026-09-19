import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    reference: {
      label: '원본 소스',
      tabs: { observations: '관찰 기록', sources: '원본 영상', requests: '요청 기록' },
    },
    steps: {
      generate: '생성',
      refine: '수정',
      finish: '완성',
      aria: '클립 단계',
      goGenerate: '생성으로 가기',
      refineWaiting: '아직 다듬을 편집안이 없어요. 먼저 클립을 생성해 주세요.',
      finishWaiting: '아직 완성된 영상이 없어요. 먼저 클립을 생성해 주세요.',
      finishDockAria: '완성된 클립 내려받기',
    },
  },
  en: {
    reference: {
      label: 'Source material',
      tabs: { observations: 'Observations', sources: 'Sources', requests: 'Requests' },
    },
    steps: {
      generate: 'Create',
      refine: 'Refine',
      finish: 'Finish',
      aria: 'Clip steps',
      goGenerate: 'Go to Create',
      refineWaiting: 'There is no edit plan to refine yet. Generate the clip first.',
      finishWaiting: 'There is no finished video yet. Generate the clip first.',
      finishDockAria: 'Download the finished clip',
    },
  },
} as const satisfies I18nFragment
