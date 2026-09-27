import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    storylineSpace: {
      title: '스토리라인',
      paragraph: '{{n}}번째 문단',
      editText: '{{n}}번째 문단 고치기',
      done: '완료',
      scene: '장면 {{n}}',
      frame: '{{scene}}의 첫 화면',
      move: '{{scene}} 옮기기',
      takeOut: '빼기',
      takenOut: '빠진 장면',
      putBack: '{{scene}} 넣기',
      putBackLabel: '넣기',
      added: '이 스토리라인을 만든 뒤 영상이 추가됐어요. 다시 만들면 새 영상도 들어가요.',
    },
  },
  en: {
    storylineSpace: {
      title: 'Storyline',
      paragraph: 'Paragraph {{n}}',
      editText: 'Edit paragraph {{n}}',
      done: 'Done',
      scene: 'Scene {{n}}',
      frame: 'First frame of {{scene}}',
      move: 'Move {{scene}}',
      takeOut: 'Take out',
      takenOut: 'Taken out',
      putBack: 'Put back {{scene}}',
      putBackLabel: 'Put back',
      added: 'Footage was added after this storyline was made. Make it again to include it.',
    },
  },
} as const satisfies I18nFragment
