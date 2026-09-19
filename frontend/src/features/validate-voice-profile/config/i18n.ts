import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    validate: {
      title: '프로필 검증',
      judge: 'AI 심사로 5개 항목의 일치율도 계산',
      action: '검증 시작',
      missing: '완성하고 학습한 글이 {{count}}편 더 필요해요.',
      missing_one: '완성하고 학습한 글이 {{count}}편 더 필요해요.',
      missing_other: '완성하고 학습한 글이 {{count}}편 더 필요해요.',
      confirmTitle: '프로필 검증을 시작할까요?',
      confirm: '검증 작업 시작',
      confirmJudgeDescription:
        '분석 모델과 작성 모델을 명시적으로 사용합니다. AI 심사 호출도 포함합니다.',
      confirmPlainDescription:
        '분석 모델과 작성 모델을 명시적으로 사용합니다. AI 심사는 호출하지 않습니다.',
    },
  },
  en: {
    validate: {
      title: 'Profile validation',
      judge: 'Also calculate agreement across five items with AI judging',
      action: 'Start validation',
      missing: '{{count}} more finalized and learned posts are required.',
      missing_one: '{{count}} more finalized and learned post is required.',
      missing_other: '{{count}} more finalized and learned posts are required.',
      confirmTitle: 'Start profile validation?',
      confirm: 'Start validation job',
      confirmJudgeDescription:
        'The selected analysis and writing models will be used explicitly. AI judging is also included.',
      confirmPlainDescription:
        'The selected analysis and writing models will be used explicitly. AI judging is not called.',
    },
  },
} as const satisfies I18nFragment
