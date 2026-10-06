import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    speech: {
      modelLabel: '목소리 생성 모델',
      choose: '모델을 선택하세요',
      unavailable: '지금은 사용할 수 없어요',
      empty: '사용할 목소리 생성 모델이 아직 없어요.',
      planRequired: '{{plan}} 요금제부터 사용할 수 있어요',
      limits: '목소리 설명 {{description}}자 · 미리듣기 {{preview}}자 · 더빙 대본 {{speech}}자까지',
      reason: {
        SPEECH_CONNECTION_UNAVAILABLE: '목소리 생성 서비스를 지금 사용할 수 없어요',
        SPEECH_PROFILE_UNAVAILABLE: '이 목소리 모델은 현재 제공되지 않아요',
        SPEECH_BINDING_INCOMPATIBLE: '저장한 목소리와 모델의 조합을 사용할 수 없어요',
        SPEECH_CATALOG_UNAVAILABLE: '모델 상태를 확인할 수 없어요. 잠시 뒤 다시 시도하세요',
        SPEECH_PATH_UNSUPPORTED: '한국어 목소리 생성과 재사용 경로를 확인해야 해요',
        SPEECH_PROFILE_CHANGED: '모델 정보가 바뀌어 다시 확인해야 해요',
        SPEECH_PRICE_UNAVAILABLE: '생성 비용의 상한을 확인할 수 없어 지금은 사용할 수 없어요',
        MODEL_UNCLASSIFIED: '운영자가 아직 등급을 분류하지 않았어요',
        SPEECH_NOT_QUALIFIED: '실제 목소리 생성 검증을 기다리고 있어요',
        MODEL_PLAN_REQUIRED: '{{plan}} 요금제부터 사용할 수 있어요',
      },
    },
    title: 'AI 모델',
    active: '활성 모델',
    vanished: '등록된 모델 목록에서 사라졌어요',
    unsuitable: '이 단계에서는 쓸 수 없는 모델이에요',
    postCredits: {
      recent: '최근 사용량 기준 글 1개당 약 {{credits}}크레딧',
      estimate: '예상 글 1개당 약 {{credits}}크레딧',
    },
    access: {
      planRequired: '{{plan}} 요금제부터 쓸 수 있어요',
      unclassified: '운영자가 아직 등급을 분류하지 않았어요',
      freePathUnavailable: '지금은 검증된 무료 공급자 경로가 없어요',
      providerUnavailable: '공급자를 지금 사용할 수 없어요',
      priceUnavailable: '가격을 확인할 수 없어 지금은 쓸 수 없어요',
      freeProviderLimit:
        '무료 모델은 공급자의 일일 제한·속도 제한·용량에 따라 일시적으로 거절될 수 있어요. 매일 사용 횟수를 보장하지 않으며, 잔액 0에서도 자동 유료 전환이나 결제는 없어요.',
      noFreeModel: '이 작업에 맞는 무료 모델이 아직 없어요.',
    },
  },
  en: {
    speech: {
      modelLabel: 'Voice creation model',
      choose: 'Choose a model',
      unavailable: 'Currently unavailable',
      empty: 'No voice creation models are available yet.',
      planRequired: 'Requires the {{plan}} plan',
      limits:
        'Description: {{description}} characters · audition: {{preview}} · spoken script: {{speech}}',
      reason: {
        SPEECH_CONNECTION_UNAVAILABLE: 'Voice creation is currently unavailable',
        SPEECH_PROFILE_UNAVAILABLE: 'This voice model is currently unavailable',
        SPEECH_BINDING_INCOMPATIBLE: 'The saved voice and model combination is incompatible',
        SPEECH_CATALOG_UNAVAILABLE: 'Model status could not be checked. Try again shortly',
        SPEECH_PATH_UNSUPPORTED: 'Korean voice creation and reuse need verification',
        SPEECH_PROFILE_CHANGED: 'Model information changed and needs verification',
        SPEECH_PRICE_UNAVAILABLE: 'A bounded generation cost could not be verified',
        MODEL_UNCLASSIFIED: 'Awaiting an operator classification',
        SPEECH_NOT_QUALIFIED: 'Waiting for live voice creation qualification',
        MODEL_PLAN_REQUIRED: 'Requires the {{plan}} plan',
      },
    },
    title: 'AI models',
    active: 'Active model',
    vanished: 'No longer appears in the registered model list',
    unsuitable: 'Cannot be used for this stage',
    postCredits: {
      recent: 'About {{credits}} credits per post, from recent usage',
      estimate: 'About {{credits}} credits per post (estimate)',
    },
    access: {
      planRequired: 'Available from the {{plan}} plan',
      unclassified: 'Awaiting an operator classification',
      freePathUnavailable: 'No verified free provider route is available right now',
      providerUnavailable: 'The provider is currently unavailable',
      priceUnavailable: 'Its price cannot be confirmed right now',
      freeProviderLimit:
        'Free models may be limited by the provider’s daily quota, rate limit, or capacity. No daily job count is guaranteed; zero balance never triggers payment or an automatic paid fallback.',
      noFreeModel: 'No compatible free model is available for this task yet.',
    },
  },
} as const satisfies I18nFragment
