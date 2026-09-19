import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16). */
export const i18n = {
  namespace: 'guidelines',
  ko: {
    title: '지침',
    page: {
      description:
        '지침은 글에서 피해야 할 내용과 주의할 점을 정해요. 저장하면 이 계정의 모든 글에 적용되고, 특정 템플릿에만 적용되게 좁힐 수도 있어요. 문체와 종결어미는 그대로 말투 프로필을 따릅니다.',
      saved: '저장된 지침',
      empty: '아직 저장된 지침이 없어요',
      emptyHelp:
        '글을 받아 보고 매번 지우던 문장을 여기에 한 번만 저장해 두세요. 예를 들어 이런 식이에요.',
      example: '무인 매장 글에서 직원·주인과의 상호작용이나 CCTV를 언급하지 않기',
      order: '지침은 전역 지침 먼저, 그다음 템플릿 지침 순서로 적용돼요.',
    },
    loadFailed: '지침 목록을 불러오지 못했어요.',
  },
  en: {
    title: 'Guidelines',
    page: {
      description:
        'A guideline says what a post must avoid or watch out for. Saved guidelines apply to every post of this account, and can be narrowed to specific templates. Tone and sentence endings still follow your voice profile.',
      saved: 'Saved guidelines',
      empty: 'No guidelines saved yet',
      emptyHelp:
        'Save the sentence you keep deleting from every draft, once, here. For example, like this.',
      example:
        'In posts about an unmanned store, do not mention staff or owner interactions, or CCTV',
      order: 'Guidelines apply global first, then the ones scoped to the post’s template.',
    },
    loadFailed: 'Could not load your guidelines.',
  },
} as const satisfies I18nFragment
