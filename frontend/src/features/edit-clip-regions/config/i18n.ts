import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    regions: {
      intro: '인트로',
      outro: '아웃트로',
      off: '사용 안 함',
      use: '{{region}} 사용',
      slot: '{{n}}번째 줄',
      instruction: '{{n}}번째 줄 들어갈 내용',
      text: '{{n}}번째 줄 문구',
      state: {
        owner: '직접 입력',
        blank: '비워 둠',
        bound: '답변에서',
        written: '생성됨',
        awaiting: '생성 대기',
      },
      nothingDrawn: '아직 영상에 보일 문구가 없어요.',
      unused: '쓰지 않는 문구',
      unusedText: '쓰지 않는 문구 {{n}}',
      moveUnused: '쓰지 않는 문구 {{n}} 옮기기',
      moveTo: '{{n}}번째 줄로 옮기기',
      move: '옮기기',
      held: '저장하지 못한 인트로·아웃트로 문구가 있어요. ②의 스토리라인에서 고쳐 주세요.',
      error: {
        copy_limit: '이 줄에 다 들어가지 않아요. 줄여 주세요.',
        unsupported_glyph: '이 줄의 글꼴로 쓸 수 없는 글자가 있어요.',
        invalid: '이 문구는 저장할 수 없어요.',
      },
    },
  },
  en: {
    regions: {
      intro: 'Intro',
      outro: 'Outro',
      off: 'Off',
      use: 'Use {{region}}',
      slot: 'Line {{n}}',
      instruction: 'Line {{n}} content',
      text: 'Line {{n}} text',
      state: {
        owner: 'Typed',
        blank: 'Left empty',
        bound: 'From an answer',
        written: 'Written',
        awaiting: 'Awaiting generation',
      },
      nothingDrawn: 'Nothing shows in the video yet.',
      unused: 'Unused text',
      unusedText: 'Unused text {{n}}',
      moveUnused: 'Move unused text {{n}}',
      moveTo: 'Move to line {{n}}',
      move: 'Move',
      held: 'Some intro or outro text could not be saved. Fix it in ②’s storyline.',
      error: {
        copy_limit: 'This does not fit on its line. Shorten it.',
        unsupported_glyph: 'This line’s typeface cannot draw some of these characters.',
        invalid: 'This text cannot be saved.',
      },
    },
  },
} as const satisfies I18nFragment
