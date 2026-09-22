import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    verdict: {
      badgesOptional: '고른 이유를 남기면 다음 비교에 도움이 돼요. 안 골라도 그대로 확정돼요.',
      chosen: '선택한 결과 {{label}}',
      unchosen: '선택하지 않은 결과 {{label}}',
      positive: '좋았던 점',
      negative: '아쉬운 점',
      noteLabel: '기타 이유',
    },
    actions: {
      retryFailed: '실패 후보 재시도',
      dismiss: '둘 다 사용하지 않기',
      retryApply: '적용 다시 시도',
      useActive: '활성 모델로 사용',
      retryAdopt: '활성 모델 변경 다시 시도',
      useSingle: '이 결과만 사용',
      choose: '이 결과로 선택',
      apply: '결과 적용',
      applyAndAdopt: '결과 적용하고 활성 모델로 변경',
      failed: '요청을 처리하지 못했어요.',
      voiceUnavailable:
        '삭제되었거나 찾을 수 없는 말투에는 결과를 적용하거나 다시 생성할 수 없어요. 먼저 복원해 주세요.',
      applyFailed: '적용하지 못했어요.',
      applied: '선택한 결과를 적용했어요.',
      adopted: ' 활성 작성 모델도 변경했어요.',
      notAdopted: ' 활성 작성 모델은 변경하지 않았어요.',
      adoptionFailed: '결과는 적용했지만 활성 작성 모델은 변경하지 못했어요.',
      confirmStyleTitle: '문체 분석 결과를 적용할까요?',
      confirmStyle: '문체 덮어쓰기',
      confirmStyleDescription:
        '현재 styleguide를 선택한 결과로 교체합니다. 직접 작성한 rules는 그대로 유지됩니다.',
    },
  },
  en: {
    verdict: {
      badgesOptional:
        'Telling us why helps the next comparison. Leaving it blank confirms just the same.',
      chosen: 'Chosen result {{label}}',
      unchosen: 'Result you did not choose, {{label}}',
      positive: 'What was good',
      negative: 'What fell short',
      noteLabel: 'Other reason',
    },
    actions: {
      retryFailed: 'Retry failed candidate',
      dismiss: 'Use neither result',
      retryApply: 'Retry applying',
      useActive: 'Use as active model',
      retryAdopt: 'Retry changing active model',
      useSingle: 'Use this result only',
      choose: 'Choose this result',
      apply: 'Apply result',
      applyAndAdopt: 'Apply result and change active model',
      failed: 'Could not process the request.',
      voiceUnavailable:
        'Results cannot be applied or regenerated for a deleted or missing voice. Restore it first.',
      applyFailed: 'Could not apply the result.',
      applied: 'Applied the selected result.',
      adopted: ' The active writing model was also changed.',
      notAdopted: ' The active writing model was not changed.',
      adoptionFailed: 'The result was applied, but the active writing model could not be changed.',
      confirmStyleTitle: 'Apply this voice-analysis result?',
      confirmStyle: 'Replace styleguide',
      confirmStyleDescription:
        'The current styleguide will be replaced by the selected result. Rules you wrote manually stay unchanged.',
    },
  },
} as const satisfies I18nFragment
