import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16): the directory's copy for a post's
 *  지침 (`page`) and for a clip's 영상 지침 (`clipPage`). */
export const i18n = {
  namespace: 'guidelines',
  ko: {
    authoringState: {
      checking: '저장하지 않은 지침 작업을 확인하고 있어요.',
      failed: '지침 편집 상태를 확인하지 못했어요. 저장된 지침은 계속 적용됩니다.',
      active: '{{kind}} “{{name}}”의 AI 작업이 진행 중이에요.',
      pending: '{{kind}} “{{name}}”의 저장 결과를 확인해야 해요.',
      conflict: '{{kind}} “{{name}}”이 다른 곳에서 바뀌었어요. 편집 내용은 유지됩니다.',
      unsavedHeading: '아직 저장하지 않은 새 지침',
      unsaved: '저장하기 전에는 글이나 영상에 적용되지 않아요.',
      createDirect: '새 지침 직접 쓰기',
      createVideoDirect: '새 영상 지침 직접 쓰기',
      savedAvailable: '저장되어 사용할 수 있어요',
      lastPublication: '마지막으로 확인한 저장: {{result}}',
    },
    page: {
      title: '지침',
      description:
        '지침은 글에서 피해야 할 내용과 주의할 점을 정해요. 저장하면 이 계정의 모든 글에 적용되고, 특정 템플릿이나 분야에만 적용되게 좁힐 수도 있어요. 문체와 종결어미는 그대로 말투 프로필을 따릅니다.',
      listAria: '지침 목록',
      empty: '아직 적용 중인 지침이 없어요',
      emptyHelp:
        '글을 받아 보고 매번 지우던 문장을 여기에 한 번만 저장해 두세요. 예를 들어 이런 식이에요.',
      example: '무인 매장 글에서 직원·주인과의 상호작용이나 CCTV를 언급하지 않기',
      order: '지침은 기본 지침, 전역 지침, 템플릿 지침, 분야 지침 순서로 적용돼요.',
      loadFailed: '지침 목록을 불러오지 못했어요.',
    },
    clipPage: {
      title: '영상 지침',
      description:
        '영상 지침은 영상의 구성과 자막에서 피해야 할 내용과 주의할 점을 정해요. 저장하면 이 계정의 모든 영상에 적용되고, 특정 영상 템플릿에만 적용되게 좁힐 수도 있어요.',
      listAria: '영상 지침 목록',
      empty: '아직 적용 중인 영상 지침이 없어요',
      emptyHelp:
        '영상을 받아 보고 매번 고치던 부분을 여기에 한 번만 저장해 두세요. 예를 들어 이런 식이에요.',
      example: '자막에 가격을 적지 않기',
      order: '영상 지침은 기본 지침, 전역 지침, 영상 템플릿 지침 순서로 적용돼요.',
      loadFailed: '영상 지침 목록을 불러오지 못했어요.',
    },
  },
  en: {
    authoringState: {
      checking: 'Checking unpublished guideline work.',
      failed: 'Could not check guideline editing status. Saved guidelines still apply.',
      active: 'AI is working on {{kind}} “{{name}}”.',
      pending: 'Confirm the save result for {{kind}} “{{name}}”.',
      conflict: '{{kind}} “{{name}}” changed elsewhere. Your editing is retained.',
      unsavedHeading: 'New guidelines not yet saved',
      unsaved: 'These do not apply to posts or videos until saved.',
      createDirect: 'Write a new guideline directly',
      createVideoDirect: 'Write a new video guideline directly',
      savedAvailable: 'Saved and available',
      lastPublication: 'Last confirmed save: {{result}}',
    },
    page: {
      title: 'Guidelines',
      description:
        'A guideline says what a post must avoid or watch out for. Saved guidelines apply to every post of this account, and can be narrowed to specific templates or categories. Tone and sentence endings still follow your voice profile.',
      listAria: 'Guidelines',
      empty: 'No guidelines in use yet',
      emptyHelp:
        'Save the sentence you keep deleting from every draft, once, here. For example, like this.',
      example:
        'In posts about an unmanned store, do not mention staff or owner interactions, or CCTV',
      order:
        'Guidelines apply the defaults first, then global ones, then those scoped to the post’s template, then those scoped to its category.',
      loadFailed: 'Could not load your guidelines.',
    },
    clipPage: {
      title: 'Video guidelines',
      description:
        'A video guideline says what a clip’s cuts and captions must avoid or watch out for. Saved video guidelines apply to every clip of this account, and can be narrowed to specific video templates.',
      listAria: 'Video guidelines',
      empty: 'No video guidelines in use yet',
      emptyHelp:
        'Save the change you keep making to every clip, once, here. For example, like this.',
      example: 'Do not put prices in the captions',
      order:
        'Video guidelines apply the defaults first, then global ones, then those scoped to the clip’s video template.',
      loadFailed: 'Could not load your video guidelines.',
    },
  },
} as const satisfies I18nFragment
