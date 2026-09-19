import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    feedback: {
      action: '문장 의견',
      title: '어떤 점을 바꾸고 싶나요?',
      confirm: '의견 남기기',
      sentence: '문장',
      reason: '이유',
      vocabulary: '단어 선택',
      ending: '종결어미',
      length: '문장 길이',
      structure: '구조',
      purpose: '이 의견은 말투를 가르칩니다. 이 글은 바뀌지 않아요.',
      changePost: '글을 고치고 싶다면 글 다듬기의 AI 수정을 사용해 주세요.',
    },
  },
  en: {
    feedback: {
      action: 'Sentence feedback',
      title: 'What would you like to change?',
      confirm: 'Send feedback',
      sentence: 'Sentence',
      reason: 'Reason',
      vocabulary: 'Word choice',
      ending: 'Sentence ending',
      length: 'Sentence length',
      structure: 'Structure',
      purpose: 'This feedback teaches the voice. It does not change this post.',
      changePost: 'To change the post itself, use Revise with AI on the Refine step.',
    },
  },
} as const satisfies I18nFragment
