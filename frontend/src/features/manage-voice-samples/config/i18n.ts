import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    samples: {
      title: '학습 샘플',
      empty: '아직 학습한 글이 없어요.',
      meta: '{{characters}}자 · {{time}}',
      meta_one: '{{characters}}자 · {{time}}',
      meta_other: '{{characters}}자 · {{time}}',
      deleteAria: '{{label}} 삭제',
      deleteTitle: '샘플을 삭제할까요?',
      deleteDescription: '“{{label}}”을(를) 지우면 남은 샘플로 문체를 다시 분석해요.',
    },
  },
  en: {
    samples: {
      title: 'Learning samples',
      empty: 'There are no learned posts yet.',
      meta: '{{characters}} characters · {{time}}',
      meta_one: '{{characters}} character · {{time}}',
      meta_other: '{{characters}} characters · {{time}}',
      deleteAria: 'Delete {{label}}',
      deleteTitle: 'Delete this sample?',
      deleteDescription: 'Deleting “{{label}}” reanalyzes the voice from the remaining samples.',
    },
  },
} as const satisfies I18nFragment
