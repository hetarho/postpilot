import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    storylineEdit: {
      paragraph: '{{n}}번째 문단',
      editText: '{{n}}번째 문단 고치기',
      done: '완료',
      move: '{{file}} 옮기기',
      takeOut: '빼기',
      takenOut: '빠진 사진',
      putBack: '{{file}} 넣기',
      putBackLabel: '넣기',
      dropHere: '여기로 옮기기',
    },
  },
  en: {
    storylineEdit: {
      paragraph: 'Paragraph {{n}}',
      editText: 'Edit paragraph {{n}}',
      done: 'Done',
      move: 'Move {{file}}',
      takeOut: 'Take out',
      takenOut: 'Taken out',
      putBack: 'Put back {{file}}',
      putBackLabel: 'Put back',
      dropHere: 'Move here',
    },
  },
} as const satisfies I18nFragment
