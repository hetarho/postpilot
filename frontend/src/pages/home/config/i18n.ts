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
        voiceLearning: '말투를 {{percent}}% 배웠어요. 내가 쓴 글을 조금 더 넣어 주세요.',
        voiceAnalyze: '말투 학습이 끝났어요. 분석하면 내 말투가 완성돼요.',
        voiceDefault: '완성된 말투 하나를 기본으로 정하면 새 글이 그 말투로 써져요.',
        templateDraft: '대화 중인 템플릿 초안이 있어요. 저장하면 준비돼요.',
        guidelineOwn: '기본 지침은 켜져 있어요. 매번 고치던 것을 내 지침으로 한 번 적어 두세요.',
        modelAgain: '쓰던 AI를 더는 쓸 수 없어요. 글을 쓸 AI를 다시 골라 주세요.',
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
        voiceLearning: 'Your style is {{percent}}% learned. Add a little more of your writing.',
        voiceAnalyze: 'Your style has learned enough. Analyse it to finish it.',
        voiceDefault: 'Make a finished style the default and new posts are written in it.',
        templateDraft: 'A template draft is waiting in a chat. Save it and it is ready.',
        guidelineOwn: 'The built-in rules are on. Write down once what you keep fixing.',
        modelAgain: 'The writing AI you chose is no longer available. Choose again.',
        next: {
          voice: 'Next is your style: show the AI how your sentences go.',
          template: 'Next is a template: settle the order of a post by talking to the AI.',
          guideline: 'Next are rules: write down once what you keep fixing.',
          memory: 'Next are memories: tell the AI facts about you.',
          model: 'Next is the writing AI: choose the model that writes.',
        },
      },
      write: 'Write',
      resume: 'Continue',
      resumeNamed: 'Continue: {{title}}',
      actions: 'Write',
    },
  },
} as const satisfies I18nFragment
