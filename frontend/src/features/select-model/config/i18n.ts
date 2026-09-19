import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const modelsI18n = {
  namespace: 'models',
  ko: {
    selectField: {
      unaffordable: '크레딧 {{credits}} 필요',
      stage: { observe: '관찰', write: '작성', analyze: '문체 분석' },
      label: '{{stage}} 모델{{optional}}',
      optional: ' (선택)',
      saving: '선택을 저장하는 중…',
      loadFailed: '모델 목록을 불러오지 못했어요.',
      saveFailed: '선택을 저장하지 못했어요. 다시 골라 주세요.',
      structuredOutput: '구조화 응답',
    },
    availability: {
      loading: '이 작업에 쓸 수 있는 모델인지 확인하는 중이에요.',
      failed: '모델을 쓸 수 있는지 확인하지 못했어요. 다시 확인해 주세요.',
      retry: '다시 확인',
      unresolved: '이 작업에서는 아직 쓸 수 없는 모델이에요.',
    },
  },
  en: {
    selectField: {
      unaffordable: 'needs {{credits}} credits',
      stage: { observe: 'Observe', write: 'Write', analyze: 'Analyze voice' },
      label: '{{stage}} model{{optional}}',
      optional: ' (optional)',
      saving: 'Saving your selection…',
      loadFailed: 'Could not load the model list.',
      saveFailed: 'Could not save your selection. Choose again.',
      structuredOutput: 'Structured output',
    },
    availability: {
      loading: 'Checking which models this task can use.',
      failed: 'Could not check whether the model can be used. Check again.',
      retry: 'Check again',
      unresolved: 'This model cannot be used for this task yet.',
    },
  },
} as const satisfies I18nFragment

/** This slice's share of the `plans` namespace (ARCH-16). */
export const plansI18n = {
  namespace: 'plans',
  ko: {
    estimate: {
      perPost: '글 1편당 약 {{credits}} 크레딧',
      posts: '남은 크레딧으로 약 {{count}}편 쓸 수 있어요',
      none: '남은 크레딧으로는 이 모델을 쓸 수 없어요',
      caveat: '사진 수와 글 길이에 따라 달라져요.',
    },
  },
  en: {
    estimate: {
      perPost: 'About {{credits}} credits per post',
      posts: 'About {{count}} posts with your remaining credits',
      none: 'Your credits do not cover this model',
      caveat: 'Varies with photo count and post length.',
    },
  },
} as const satisfies I18nFragment
