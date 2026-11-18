import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `creation` namespace (ARCH-16): the words of the offer, which the
 *  writing kit draws (`widgets/writing-kit`'s `CreationOfferSheet`). */
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
