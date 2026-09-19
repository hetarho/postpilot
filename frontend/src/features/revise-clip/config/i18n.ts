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
