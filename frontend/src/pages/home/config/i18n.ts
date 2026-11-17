import type { I18nFragment } from '@/shared/lib'

/** 홈: the promise, the kit's readiness, and the one 글쓰기. */
export const i18n = {
  namespace: 'creation',
  ko: {
    home: {
      title: '홈',
      eyebrow: '글쓰기 준비 {{ready}} / {{total}}',
      promiseLead: '고품질의 글을,',
      promiseAccent: '내 언어로.',
      phase: {
        empty: '말투 · 템플릿 · 지침 · 기억을 채울수록 AI가 내 글처럼 써요.',
        ready: '모두 준비됐어요. 이제 내 언어로 써 볼까요?',
        next: {
          voice: '다음은 말투예요. 내가 쓴 글로 문장 습관을 알려 주세요.',
          template: '다음은 템플릿이에요. AI와 이야기하며 글의 순서를 정해요.',
          guideline: '다음은 지침이에요. 매번 고치던 것을 한 번만 적어 두세요.',
          memory: '다음은 기억이에요. 나에 대한 사실을 알려 주세요.',
          model: '다음은 쓰는 AI예요. 글을 쓸 모델을 골라 주세요.',
        },
      },
      write: '글쓰기',
      resume: '이어 쓰기',
      resumeNamed: '이어 쓰기: {{title}}',
      actions: '글쓰기',
    },
  },
  en: {
    home: {
      title: 'Home',
      eyebrow: 'Ready to write {{ready}} / {{total}}',
      promiseLead: 'High-quality posts,',
      promiseAccent: 'in your own words.',
      phase: {
        empty:
          'The more of your style, templates, rules and memories you add, the more it reads like you.',
        ready: 'Everything is ready. Shall we write in your words?',
        next: {
          voice: 'Next is your style: show the AI how your sentences go.',
          template: 'Next is a template: settle the order of a post by talking to the AI.',
          guideline: 'Next are rules: write down once what you keep fixing.',
          memory: 'Next are memories: tell the AI facts about you.',
          model: 'Next is the writing AI: choose the model that writes.',
        },
      },
      write: 'Write a post',
      resume: 'Continue',
      resumeNamed: 'Continue: {{title}}',
      actions: 'Write',
    },
  },
} as const satisfies I18nFragment
