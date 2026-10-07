import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    navigation: {
      returnCreation: '만들기로 돌아가기',
      returnHistory: '작업 내역으로 돌아가기',
      untitled: '제목 없는 클립',
    },
    editorEntries: {
      storyline: '스토리라인',
      script: '더빙 대본',
      ai: 'AI로 수정',
      scriptHelp: '영상에서 읽을 대본과 화면에 표시할 자막은 따로 편집할 수 있어요.',
      noScript: '더빙할 목소리와 대본을 준비해 주세요.',
      voices: '내 목소리 보기',
    },
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
      refineWaiting:
        '아직 다듬을 편집안이 없어요. ①에서 스토리라인 먼저나 바로 만들기로 시작해 주세요.',
      finishWaiting:
        '아직 완성된 영상이 없어요. ①에서 스토리라인 먼저나 바로 만들기로 시작해 주세요.',
      finishDockAria: '완성된 클립 내려받기',
    },
  },
  en: {
    navigation: {
      returnCreation: 'Back to creation',
      returnHistory: 'Back to work history',
      untitled: 'Untitled clip',
    },
    editorEntries: {
      storyline: 'Storyline',
      script: 'Spoken script',
      ai: 'Edit with AI',
      scriptHelp: 'Edit the spoken script separately from displayed captions.',
      noScript: 'Prepare a confirmed voice and spoken script for this video.',
      voices: 'My spoken voices',
    },
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
      refineWaiting:
        'There is no edit plan to refine yet. Start in ① with Storyline first or Make now.',
      finishWaiting: 'There is no finished video yet. Start in ① with Storyline first or Make now.',
      finishDockAria: 'Download the finished clip',
    },
  },
} as const satisfies I18nFragment
