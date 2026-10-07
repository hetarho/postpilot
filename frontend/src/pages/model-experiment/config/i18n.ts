import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    experiment: {
      loading: '비교 결과를 불러오는 중…',
      legacyRunning: '이미 시작한 유료 작업은 계속 진행돼요. 이 화면을 닫아도 취소되지 않아요.',
      legacyExpired: '보관 기간이 지나 삭제된 결과는 열람하거나 복사할 수 없어요.',
      copyResult: '결과 {{label}} 복사',
      copied: '결과 {{label}}를 복사했어요.',
      copyFallback: '자동 복사를 사용할 수 없어요. 아래 결과를 선택해서 복사해 주세요.',
      loadFailed: '비교 결과를 불러오지 못했어요.',
      backPost: '← 글로 돌아가기',
      backModels: '← 글쓰기 테스트 기록',
      backComparison: '← 글쓰기 테스트로 돌아가기',
      backPosts: '← 내 글 목록으로 돌아가기',
      title: '이전 유료 비교 기록',
      description:
        '저장된 원본 결과를 보여 드려요. 이전 비교에는 새 토너먼트나 우승 결과를 만들지 않아요.',
      template: '템플릿 · {{name}}',
      voice: '말투 · {{name}}',
      pieceLabel: '이 후보',
      language: '고정된 글 언어',
      actionAria: '이전 결과 읽기',
      selectAria: '읽을 결과',
    },
  },
  en: {
    experiment: {
      loading: 'Loading comparison results…',
      legacyRunning:
        'Previously started paid work continues. Closing this screen does not cancel it.',
      legacyExpired:
        'Results removed after their retention period can no longer be read or copied.',
      copyResult: 'Copy result {{label}}',
      copied: 'Copied result {{label}}.',
      copyFallback: 'Automatic copying is unavailable. Select and copy the result below.',
      loadFailed: 'Could not load comparison results.',
      backPost: '← Back to post',
      backModels: '← Writing test history',
      backComparison: '← Back to writing tests',
      backPosts: '← Back to my posts',
      title: 'Earlier paid comparison',
      description:
        'These are the retained original results. An earlier comparison does not acquire a new tournament or champion.',
      template: 'Template · {{name}}',
      voice: 'Voice · {{name}}',
      pieceLabel: 'This candidate',
      language: 'Frozen post language',
      actionAria: 'Read earlier results',
      selectAria: 'Result to read',
    },
  },
} as const satisfies I18nFragment
