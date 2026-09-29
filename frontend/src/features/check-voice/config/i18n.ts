import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    check: {
      open: '검증하기',
      title: '검증할 문항',
      answered: '답함',
      photo: '사진',
      photoNeedsVision: '사진을 읽는 작성 모델에서만 검증할 수 있어요.',
      noModel: '검증에 쓸 작성 모델을 먼저 골라 주세요.',
      chooseModel: 'AI 모델 고르기',
      notMade: '말투를 만든 뒤 검증할 수 있어요.',
      deleted: '삭제된 말투는 검증할 수 없어요. 먼저 복원해 주세요.',
      loadFailed: '문항을 불러오지 못했어요.',
      retry: '다시 검증',
    },
  },
  en: {
    check: {
      open: 'Check the voice',
      title: 'Prompt to check',
      answered: 'Answered',
      photo: 'Photo',
      photoNeedsVision: 'Only a writing model that reads photos can check this one.',
      noModel: 'Choose a writing model for the check first.',
      chooseModel: 'Choose an AI model',
      notMade: 'Make the voice before checking it.',
      deleted: 'A deleted voice cannot be checked. Restore it first.',
      loadFailed: 'Could not load the prompts.',
      retry: 'Check again',
    },
  },
} as const satisfies I18nFragment
