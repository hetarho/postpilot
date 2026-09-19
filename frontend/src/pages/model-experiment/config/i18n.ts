import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    experiment: {
      loading: '비교 결과를 불러오는 중…',
      loadFailed: '비교 결과를 불러오지 못했어요.',
      backPost: '← 글로 돌아가기',
      backModels: '← AI 모델',
      title: '블라인드 비교',
      description:
        '선택하기 전에는 모델 이름과 비용을 숨깁니다. 좌우 후보는 다시 열어도 바뀌지 않습니다.',
      template: '템플릿 · {{name}}',
      voice: '말투 · {{name}}',
      language: '고정된 글 언어',
      actionAria: '후보 전환과 결정',
      selectAria: '선택할 후보',
    },
  },
  en: {
    experiment: {
      loading: 'Loading comparison results…',
      loadFailed: 'Could not load comparison results.',
      backPost: '← Back to post',
      backModels: '← AI models',
      title: 'Blind comparison',
      description:
        'Model names and costs stay hidden until you choose. The left and right candidates remain stable when reopened.',
      template: 'Template · {{name}}',
      voice: 'Voice · {{name}}',
      language: 'Frozen post language',
      actionAria: 'Switch and decide candidates',
      selectAria: 'Candidate to select',
    },
  },
} as const satisfies I18nFragment
