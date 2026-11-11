import type { I18nFragment } from '@/shared/lib'

/** Guide: 내 스타일 — everything the owner has taught the AI, named by what it changes. */
export const i18n = {
  namespace: 'creation',
  ko: {
    guideStyle: {
      title: '내 스타일',
      lead: 'AI에게 알려준 것',
      sections: '알려준 것',
      voices: { name: '말투', outcome: '글이 내 말투로 써져요', count: '{{count}}개' },
      templates: { name: '템플릿', outcome: '글과 영상이 이 순서로 짜여요', count: '{{count}}개' },
      guidelines: { name: '지침', outcome: 'AI가 항상 지킬 것', count: '{{count}}개' },
      memories: { name: '기억', outcome: '가게 이야기를 기억해 둬요', count: '{{count}}개' },
      spoken: { name: '목소리', outcome: '영상을 이 목소리로 읽어 줘요' },
      defaultVoice: '기본 · {{name}}',
    },
  },
  en: {
    guideStyle: {
      title: 'My style',
      lead: 'What you taught the AI',
      sections: 'What you taught',
      voices: { name: 'Styles', outcome: 'Posts are written in your voice', count: '{{count}}' },
      templates: {
        name: 'Templates',
        outcome: 'Posts and videos follow this order',
        count: '{{count}}',
      },
      guidelines: { name: 'Rules', outcome: 'What the AI always keeps to', count: '{{count}}' },
      memories: { name: 'Memories', outcome: 'Your shop’s story, remembered', count: '{{count}}' },
      spoken: { name: 'Voices', outcome: 'Videos are read in this voice' },
      defaultVoice: 'Default · {{name}}',
    },
  },
} as const satisfies I18nFragment
