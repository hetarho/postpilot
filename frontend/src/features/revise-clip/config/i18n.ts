import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    revision: {
      request: '요청 내용',
      count: '{{used}} / {{max}}자',
      target: '고칠 대상',
      targets: {
        flow: '영상 흐름',
        narration: '자막',
        both: '둘 다',
      },
      send: 'AI에 수정 요청',
      approve: '최대 {{amount, number}} 크레딧 · 승인하고 수정 요청',
      running: '수정안을 쓰는 중이에요',
      readOnly:
        '수정이 끝날 때까지 타임라인은 읽기 전용이에요. 미리보기와 원본은 그대로 볼 수 있어요.',
      cancelled: '수정 요청을 중단했어요. 편집안과 영상은 그대로예요.',
    },
  },
  en: {
    revision: {
      request: 'What to change',
      count: '{{used}} / {{max}} characters',
      target: 'What to revise',
      targets: {
        flow: 'Footage flow',
        narration: 'Captions',
        both: 'Both',
      },
      send: 'Ask the AI to revise',
      approve: 'Up to {{amount, number}} credits · approve and ask',
      running: 'Writing the revision',
      readOnly:
        'The timeline is read-only until the revision finishes. The preview and the sources stay open.',
      cancelled: 'The revision was stopped. The edit plan and the clip are unchanged.',
    },
  },
} as const satisfies I18nFragment

/** This slice's share of the `guidelines` namespace (ARCH-16). */
export const guidelinesI18n = {
  namespace: 'guidelines',
  ko: {
    clipCapture: {
      action: '영상 지침으로 저장',
      description:
        '이 수정 요청을 다음 영상에도 계속 적용할 규칙으로 저장해요. 저장 전에 고칠 수 있어요.',
      text: '영상 지침',
      scope: '적용 범위',
      scopeGlobal: '전역',
      scopeTemplate: '이 영상의 템플릿 「{{name}}」에만',
      submit: '저장',
      saved: '영상 지침으로 저장했어요.',
      duplicate: '이미 같은 영상 지침이 있어요.',
    },
  },
  en: {
    clipCapture: {
      action: 'Save as video guideline',
      description:
        'Save this revision request as a rule to keep applying to future clips. You can edit it before saving.',
      text: 'Video guideline',
      scope: 'Applies to',
      scopeGlobal: 'Everything',
      scopeTemplate: 'Only this clip’s template “{{name}}”',
      submit: 'Save',
      saved: 'Saved as a video guideline.',
      duplicate: 'You already have the same video guideline.',
    },
  },
} as const satisfies I18nFragment
