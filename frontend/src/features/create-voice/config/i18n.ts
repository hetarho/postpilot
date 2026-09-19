import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    create: {
      open: '새 말투 만들기',
      dockAria: '말투 추가',
      title: '새 말투',
      name: '말투 이름',
      placeholder: '예: 제품 리뷰',
      submit: '말투 만들기',
      count: '{{count}} / {{max}}자',
      count_one: '{{count}} / {{max}}자',
      count_other: '{{count}} / {{max}}자',
      emptyHelp: '새 말투는 빈 프로필로 시작하고, 다른 말투와 아무것도 공유하지 않아요.',
      sourceLanguage: '샘플 언어',
      sourceLanguageHelp: '이 말투가 배울 글의 언어예요. 만든 뒤에는 바꿀 수 없어요.',
      description: '말투 설명 (선택)',
      descriptionPlaceholder: '예: 단순하고 차분하지 않은, 농담조의 요리 말투',
      descriptionCount: '{{count}} / {{max}}자',
      descriptionCount_one: '{{count}} / {{max}}자',
      descriptionCount_other: '{{count}} / {{max}}자',
      descriptionHelp:
        '원하는 말투를 문장으로 적으면 AI가 첫 프로필을 만들어 줘요. 비워 두면 빈 프로필로 시작해요.',
      descriptionNeedsModel:
        '말투 분석에 쓸 AI 모델을 먼저 골라야 설명으로 만들 수 있어요. 설명을 비우면 지금 바로 만들 수 있어요.',
    },
  },
  en: {
    create: {
      open: 'New voice',
      dockAria: 'Add a voice',
      title: 'New voice',
      name: 'Voice name',
      placeholder: 'For example: Product reviews',
      submit: 'Create voice',
      count: '{{count}} / {{max}} characters',
      count_one: '{{count}} / {{max}} character',
      count_other: '{{count}} / {{max}} characters',
      emptyHelp: 'A new voice starts with an empty profile and shares nothing with other voices.',
      sourceLanguage: 'Sample language',
      sourceLanguageHelp:
        'This is the language the voice learns from. It cannot be changed after creation.',
      description: 'Describe the voice (optional)',
      descriptionPlaceholder: 'For example: a plain, restless, joking voice for cooking posts',
      descriptionCount: '{{count}} / {{max}} characters',
      descriptionCount_one: '{{count}} / {{max}} character',
      descriptionCount_other: '{{count}} / {{max}} characters',
      descriptionHelp:
        'Describe the register you want and the AI writes the first profile from it. Leave it empty to start with an empty profile.',
      descriptionNeedsModel:
        'Choose an analysis model first to create a voice from a description. You can still create one now by leaving the description empty.',
    },
  },
} as const satisfies I18nFragment
