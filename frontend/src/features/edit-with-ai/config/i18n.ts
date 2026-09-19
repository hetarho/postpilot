import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16). */
export const guidelinesI18n = {
  namespace: 'guidelines',
  ko: {
    capture: {
      action: '지침으로 저장',
      title: '지침으로 저장',
      description:
        '이 수정 요청을 다음 글에도 계속 적용할 규칙으로 저장해요. 저장 전에 고칠 수 있어요.',
      scopeGlobal: '전역',
      scopeTemplate: '이 글의 템플릿 「{{name}}」에만',
      submit: '저장',
      saved: '지침으로 저장했어요.',
      duplicate: '이미 같은 지침이 있어요.',
    },
  },
  en: {
    capture: {
      action: 'Save as guideline',
      title: 'Save as guideline',
      description:
        'Save this revision instruction as a rule to keep applying to future posts. You can edit it before saving.',
      scopeGlobal: 'Everything',
      scopeTemplate: 'Only this post’s template “{{name}}”',
      submit: 'Save',
      saved: 'Saved as a guideline.',
      duplicate: 'You already have the same guideline.',
    },
  },
} as const satisfies I18nFragment

/** This slice's share of the `posts` namespace (ARCH-16). */
export const postsI18n = {
  namespace: 'posts',
  ko: {
    revision: {
      title: 'AI로 수정',
      instruction: '수정 요청을 입력하세요',
      placeholder: '어떻게 고칠까요? 예: 더 짧게 · 존댓말로 · 카페 얘기 늘려줘',
      saveAsRule: '이 요청을 규칙으로 저장',
      ruleLanguageMismatch:
        '현재 본문과 말투의 샘플 언어가 달라 규칙으로 저장할 수 없어요. AI 수정 자체는 계속할 수 있습니다.',
      submit: '수정',
      prepareFailed: '편집한 글을 먼저 저장하지 못했어요.',
      blocked: {
        jobChecking: '수정 작업을 확인하는 중이에요.',
        activeJob: '다른 작업이 진행 중이에요.',
        modelChecking: '작성 모델을 확인하는 중이에요.',
        model: '글 생성 단계의 글쓰기 옵션에서 작성 모델을 선택하세요.',
      },
    },
  },
  en: {
    revision: {
      title: 'Revise with AI',
      instruction: 'Enter a revision request',
      placeholder:
        'What should change? For example: make it shorter, use a formal tone, or expand the cafe section',
      saveAsRule: 'Save this request as a rule',
      ruleLanguageMismatch:
        'The content and voice sample languages differ, so this cannot be saved as a rule. AI revision is still available.',
      submit: 'Revise',
      prepareFailed: 'Could not save your edits before starting the revision.',
      blocked: {
        jobChecking: 'Checking the revision job…',
        activeJob: 'Another job is in progress.',
        modelChecking: 'Checking the writing model…',
        model: 'Choose a writing model in the writing options on the Generate step.',
      },
    },
  },
} as const satisfies I18nFragment
