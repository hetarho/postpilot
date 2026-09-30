import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    title: 'AI 모델',
    active: '활성 모델',
    vanished: '등록된 모델 목록에서 사라졌어요',
    unsuitable: '이 단계에서는 쓸 수 없는 모델이에요',
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
    title: 'AI models',
    active: 'Active model',
    vanished: 'No longer appears in the registered model list',
    unsuitable: 'Cannot be used for this stage',
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
