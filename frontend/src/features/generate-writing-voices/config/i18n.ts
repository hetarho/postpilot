import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'voices',
  ko: {
    candidateFlow: {
      open: 'AI 말투 추천받기',
      dismiss: '닫기',
      title: '마음에 드는 말투를 골라 볼까요?',
      intro:
        'AI가 서로 다른 느낌의 말투 8개를 만들어요. 예시를 읽고 마음에 드는 스타일 하나를 골라 주세요.',
      loading: '이전에 만든 스타일을 확인하고 있어요.',
      loadFailed: '스타일을 불러오지 못했어요.',
      generate: '스타일 8개 만들어 보기',
      reroll: '다른 스타일 8개 만들기',
      ready:
        '같은 가상의 상황을 8가지 말투로 썼어요. 선택한 스타일은 나중에 설정에서 바꿀 수 있어요.',
      adopt: '이 말투 사용하기',
      selectFirst: '마음에 드는 스타일을 하나 골라 주세요.',
      confirming: '새로운 스타일 8개를 만들까요?',
      quote: '이번 생성의 예상 사용량은 {{credits, number}}크레딧이에요.',
      free: '이번 생성은 무료예요.',
      actual:
        '완료되거나 중단될 때 확인된 사용량만 정산해요. 새로 만들기를 누를 때마다 별도의 생성이에요.',
      estimating: '사용할 크레딧을 확인하고 있어요.',
      estimateFailed: '사용할 크레딧을 확인하지 못했어요.',
      confirm: '8개 만들기',
      close: '돌아가기',
      retry: '다시 확인하기',
      running: '서로 다른 말투 8개를 만들고 있어요.',
      cancelling: '생성을 중단하고 있어요.',
      retained: '기존 스타일은 그대로 있어요. 새 스타일이 완성되면 함께 보여 드릴게요.',
      failed: '새 스타일을 완성하지 못했어요. 다시 만들거나 기존 스타일을 골라도 괜찮아요.',
      unavailable: 'AI를 준비하지 못했어요. 다시 확인하거나 설정에서 도움을 받을 수 있어요.',
      preparing: 'AI를 준비하고 있어요.',
      settings: 'AI 설정 확인하기',
      cancel: '생성 중단하기',
      cancelTitle: '말투 생성을 중단할까요?',
      cancelExplanation:
        '이미 진행한 AI 작업에서 확인된 사용량만 정산해요. 전에 완성한 스타일은 남아 있어요.',
      keepRunning: '계속 만들기',
      statusFailed: '진행 상황을 확인하지 못했어요. 생성을 다시 시작하지 않고 확인할 수 있어요.',
    },
  },
  en: {
    candidateFlow: {
      open: 'Find an AI writing style',
      dismiss: 'Close',
      title: 'Find a writing style you like',
      intro:
        'AI creates eight contrasting styles. Read the examples and choose one that feels right.',
      loading: 'Checking your previous styles.',
      loadFailed: 'Could not load styles.',
      generate: 'Create eight styles',
      reroll: 'Create eight different styles',
      ready:
        'Eight versions of the same fictional scene. You can change your chosen style later in settings.',
      adopt: 'Use this style',
      selectFirst: 'Choose a style you like.',
      confirming: 'Create eight new styles?',
      quote: 'Estimated use for this generation: {{credits, number}} credits.',
      free: 'This generation is free.',
      actual:
        'Only confirmed usage is settled when work finishes or stops. Each new batch is a separate generation.',
      estimating: 'Checking the credit limit.',
      estimateFailed: 'Could not check the credit limit.',
      confirm: 'Create eight',
      close: 'Go back',
      retry: 'Check again',
      running: 'Creating eight contrasting writing styles.',
      cancelling: 'Stopping generation.',
      retained:
        'Your previous styles remain available. The new styles will appear together when complete.',
      failed:
        'The new styles could not be completed. Create another batch or choose a previous style.',
      unavailable: 'AI is not ready. Check again or get help in settings.',
      preparing: 'Preparing AI.',
      settings: 'Check AI settings',
      cancel: 'Stop generation',
      cancelTitle: 'Stop creating styles?',
      cancelExplanation:
        'Only confirmed usage from work already performed is settled. Your previously completed styles remain.',
      keepRunning: 'Keep creating',
      statusFailed: 'Could not check progress. Check again without restarting generation.',
    },
  },
} as const satisfies I18nFragment
