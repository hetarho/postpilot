import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    observation: {
      title: '사진 관찰',
      description: '모델이 각 사진에서 본 것입니다. 글이 이상하면 여기부터 확인하세요',
      position: '{{current}} / {{total}}',
      imageAlt: '{{filename}} 관찰 사진',
      urlPending: '사진 주소를 준비하는 중…',
      scene: '장면',
      mood: '분위기',
      visibleText: '보이는 글자',
      objects: '사물',
      waiting: '관찰 대기',
      empty: '관찰 결과 없음',
      events: '일어난 일',
      speech: '들린 말',
      eventsMore: '{{count}}개 더 보기',
      eventsLess: '접기',
      none: '없음',
    },
  },
  en: {
    observation: {
      title: 'Photo observations',
      description: 'What the model saw in each photo. Start here if the post looks wrong',
      position: '{{current}} / {{total}}',
      imageAlt: 'Observation photo: {{filename}}',
      urlPending: 'Preparing the photo URL…',
      scene: 'Scene',
      mood: 'Mood',
      visibleText: 'Visible text',
      objects: 'Objects',
      waiting: 'Waiting for observation',
      empty: 'No observation result',
      events: 'What happens',
      speech: 'What is said',
      eventsMore: 'Show {{count}} more',
      eventsLess: 'Show less',
      none: 'None',
    },
  },
} as const satisfies I18nFragment
