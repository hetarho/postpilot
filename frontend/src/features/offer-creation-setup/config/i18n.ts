import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `creation` namespace (ARCH-16). */
export const i18n = {
  namespace: 'creation',
  ko: {
    offer: {
      title: {
        both: '말투와 템플릿이 아직 없어요',
        voice: '말투가 아직 없어요',
        template: '템플릿이 아직 없어요',
        clip: '영상 템플릿이 아직 없어요',
      },
      description: {
        post: '먼저 만들어 두면 글마다 골라 쓸 수 있어요.',
        clip: '먼저 만들어 두면 영상마다 골라 쓸 수 있어요.',
      },
      createVoice: '새 말투 만들기',
      continueVoice: '말투 이어서 만들기',
      createTemplate: '새 템플릿 만들기',
      createClipTemplate: '새 영상 템플릿 만들기',
      close: {
        post: '이대로 쓰기',
        clip: '이대로 만들기',
      },
      dismiss: '다시 묻지 않기',
    },
  },
  en: {
    offer: {
      title: {
        both: 'No voice or template yet',
        voice: 'No voice yet',
        template: 'No template yet',
        clip: 'No video template yet',
      },
      description: {
        post: 'Make one first and pick it for any post.',
        clip: 'Make one first and pick it for any clip.',
      },
      createVoice: 'New voice',
      continueVoice: 'Keep making my voice',
      createTemplate: 'New template',
      createClipTemplate: 'New video template',
      close: {
        post: 'Write without',
        clip: 'Make without',
      },
      dismiss: 'Don’t ask again',
    },
  },
} as const satisfies I18nFragment
