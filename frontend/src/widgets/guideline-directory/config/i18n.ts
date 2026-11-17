import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16): the directory's copy for a post's
 *  지침 (`page`) and for a clip's 영상 지침 (`clipPage`). */
export const i18n = {
  namespace: 'guidelines',
  ko: {
    layers: {
      defaults: {
        title: '기본 지침',
        how: 'Postpilot이 권하는 규칙이에요. 고르거나 끌 수 있어요',
        choose: '기본 지침 고르기',
      },
      global: { title: '모든 글에', how: '내가 쓴 규칙 — 어떤 글이든 지켜요' },
      clipGlobal: { title: '모든 영상에', how: '내가 쓴 규칙 — 어떤 영상이든 지켜요' },
      templates: { title: '이 템플릿으로 쓸 때', how: '그 템플릿으로 쓰는 글에서만 지켜요' },
      fields: { title: '이 분야의 글에', how: '그 분야의 글에서만 지켜요' },
      none: '아직 없어요',
      ownEmpty:
        '내가 쓴 규칙이 아직 없어요. 받아 본 글에서 매번 고치던 부분을 한 번만 적어 두세요.',
      newMenu: '새 지침',
      methodAi: 'AI와 대화로 만들기',
      methodAiHelp: '바라는 점을 말하면 AI가 규칙 문장으로 다듬어요',
      methodDirect: '직접 쓰기',
    },
    statement: 'AI가 글을 쓸 때 늘 지키는 규칙이에요. 위에서부터 차례로 겹쳐 적용돼요.',
    clipStatement: 'AI가 영상을 짤 때 늘 지키는 규칙이에요. 위에서부터 차례로 겹쳐 적용돼요.',
    principles: {
      title: 'Postpilot 원칙',
      summary: '항상 적용돼요',
      help: '원칙은 끌 수 없고, 모든 글과 영상에 다른 지침보다 먼저 적용돼요. 겪은 것만, 솔직하게 쓰기 위한 약속이에요.',
      listAria: 'Postpilot 원칙 목록',
    },
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
      always: 'AI가 항상 지킬 것',
      description:
        '지침은 글에서 피해야 할 내용과 주의할 점을 정해요. 저장하면 이 계정의 모든 글에 적용되고, 특정 템플릿이나 분야에만 적용되게 좁힐 수도 있어요. 문체와 종결어미는 그대로 말투 프로필을 따릅니다.',
      listAria: '지침 목록',
      empty: '아직 적용 중인 지침이 없어요',
      emptyHelp:
        '글을 받아 보고 매번 지우던 문장을 여기에 한 번만 저장해 두세요. 예를 들어 이런 식이에요.',
      example: '무인 매장 글에서 직원·주인과의 상호작용이나 CCTV를 언급하지 않기',
      order: '지침은 원칙, 기본 지침, 전역 지침, 템플릿 지침, 분야 지침 순서로 적용돼요.',
      loadFailed: '지침 목록을 불러오지 못했어요.',
    },
    clipPage: {
      title: '영상 지침',
      always: '영상에서 AI가 항상 지킬 것',
      description:
        '영상 지침은 영상의 구성과 자막에서 피해야 할 내용과 주의할 점을 정해요. 저장하면 이 계정의 모든 영상에 적용되고, 특정 영상 템플릿에만 적용되게 좁힐 수도 있어요.',
      listAria: '영상 지침 목록',
      empty: '아직 적용 중인 영상 지침이 없어요',
      emptyHelp:
        '영상을 받아 보고 매번 고치던 부분을 여기에 한 번만 저장해 두세요. 예를 들어 이런 식이에요.',
      example: '자막에 가격을 적지 않기',
      order: '영상 지침은 원칙, 기본 지침, 전역 지침, 영상 템플릿 지침 순서로 적용돼요.',
      loadFailed: '영상 지침 목록을 불러오지 못했어요.',
    },
  },
  en: {
    layers: {
      defaults: {
        title: 'Default rules',
        how: 'Rules Postpilot recommends. Pick or switch them off',
        choose: 'Choose default rules',
      },
      global: { title: 'Every post', how: 'Your own rules — kept in every post' },
      clipGlobal: { title: 'Every video', how: 'Your own rules — kept in every video' },
      templates: { title: 'With a template', how: 'Kept only in posts written with that template' },
      fields: { title: 'In a field', how: 'Kept only in posts of that field' },
      none: 'Nothing yet',
      ownEmpty:
        'No rules of your own yet. Write down, once, what you keep fixing in the posts you get.',
      newMenu: 'New rule',
      methodAi: 'Make it by talking to AI',
      methodAiHelp: 'Say what you want and the AI turns it into a rule',
      methodDirect: 'Write it myself',
    },
    statement: 'The rules the AI always keeps while writing. They stack in order, top first.',
    clipStatement:
      'The rules the AI always keeps while making a video. They stack in order, top first.',
    principles: {
      title: 'Postpilot principles',
      summary: 'Always applied',
      help: 'Principles cannot be switched off and apply to every post and clip before any other guideline. They are the promise to write only what you experienced, honestly.',
      listAria: 'Postpilot principles',
    },
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
      always: 'What the AI always keeps to',
      description:
        'A guideline says what a post must avoid or watch out for. Saved guidelines apply to every post of this account, and can be narrowed to specific templates or categories. Tone and sentence endings still follow your voice profile.',
      listAria: 'Guidelines',
      empty: 'No guidelines in use yet',
      emptyHelp:
        'Save the sentence you keep deleting from every draft, once, here. For example, like this.',
      example:
        'In posts about an unmanned store, do not mention staff or owner interactions, or CCTV',
      order:
        'Guidelines apply the principles first, then the defaults, then global ones, then those scoped to the post’s template, then those scoped to its category.',
      loadFailed: 'Could not load your guidelines.',
    },
    clipPage: {
      title: 'Video guidelines',
      always: 'What the AI always keeps to in videos',
      description:
        'A video guideline says what a clip’s cuts and captions must avoid or watch out for. Saved video guidelines apply to every clip of this account, and can be narrowed to specific video templates.',
      listAria: 'Video guidelines',
      empty: 'No video guidelines in use yet',
      emptyHelp:
        'Save the change you keep making to every clip, once, here. For example, like this.',
      example: 'Do not put prices in the captions',
      order:
        'Video guidelines apply the principles first, then the defaults, then global ones, then those scoped to the clip’s video template.',
      loadFailed: 'Could not load your video guidelines.',
    },
  },
} as const satisfies I18nFragment
