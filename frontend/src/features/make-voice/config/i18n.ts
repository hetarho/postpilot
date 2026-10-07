import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    make: {
      first: '말투 만들기',
      checkState: '말투 상태 확인',
      confirmFirst: '이 자료로 말투를 분석할까요?',
      confirmAgain: '변경한 자료를 다시 분석할까요?',
      startFirst: '분석 시작',
      startAgain: '다시 분석 시작',
      model: '분석 AI: {{model}}',
      free: '무료로 분석할 수 있어요.',
      estimateFailed: '분석 예상 비용을 확인하지 못했어요. 다시 확인해 주세요.',
      credits: '예상 {{credits}} 크레딧',
      previous: '분석이 완료될 때까지는 이전에 받아 둔 말투를 계속 사용해요.',
      uncertain:
        '분석 요청 결과를 확인 중이에요. 새 요청을 만들기 전에 말투 상태를 다시 확인해 주세요.',
      again: '다시 분석',
      notReady: '말투 학습에 필요한 정보가 100%가 되면 누를 수 있어요.',
      noModel: '말투 분석에 쓸 AI 모델을 먼저 골라 주세요.',
      chooseModel: 'AI 모델 고르기',
      deleted: '삭제된 말투는 분석할 수 없어요. 먼저 복원해 주세요.',
    },
  },
  en: {
    make: {
      first: 'Make the voice',
      checkState: 'Check writing style state',
      confirmFirst: 'Analyze this learning material?',
      confirmAgain: 'Reanalyze the changed material?',
      startFirst: 'Start analysis',
      startAgain: 'Start reanalysis',
      model: 'Analysis AI: {{model}}',
      free: 'This analysis is free.',
      estimateFailed: 'The analysis estimate is unavailable. Check again.',
      credits: 'Estimated {{credits}} credits',
      previous: 'The previously accepted writing style stays in use until analysis completes.',
      uncertain: 'Checking the analysis request. Confirm its outcome before starting new work.',
      again: 'Analyze again',
      notReady: 'Available once what the voice needs reaches 100%.',
      noModel: 'Choose an AI model for voice analysis first.',
      chooseModel: 'Choose an AI model',
      deleted: 'A deleted voice cannot be analyzed. Restore it first.',
    },
  },
} as const satisfies I18nFragment
