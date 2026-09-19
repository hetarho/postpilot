import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    unavailable: '등록 해제된 모델',
    leaderboard: {
      empty: '아직 비교 결과가 없어요.',
      collecting: '데이터 수집 중',
      active: '활성',
      recommended: '추천',
      disappeared: '등록 해제',
      record: '{{matches}}전 {{wins}}승 {{losses}}패 · 승률 {{rate}}%',
      metrics:
        '성공 호출 {{calls}} · 평균 {{latency}}ms · 토큰 {{prompt}} / {{completion}} · {{cost}}',
      costUnavailable: '비용 미제공',
      partlyEstimated: '일부 ≈ ',
    },
  },
  en: {
    unavailable: 'Unregistered model',
    leaderboard: {
      empty: 'There are no comparison results yet.',
      collecting: 'Collecting data',
      active: 'Active',
      recommended: 'Recommended',
      disappeared: 'Unregistered',
      record: '{{matches}} matches, {{wins}} wins, {{losses}} losses · {{rate}}% win rate',
      metrics:
        '{{calls}} successful calls · {{latency}}ms average · tokens {{prompt}} / {{completion}} · {{cost}}',
      costUnavailable: 'Cost unavailable',
      partlyEstimated: 'partly ≈ ',
    },
  },
} as const satisfies I18nFragment
