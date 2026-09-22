import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    unavailable: '등록 해제된 모델',
    leaderboard: {
      empty: '아직 비교 결과가 없어요.',
      emptyIn: {
        day: '최근 24시간 안에는 비교 결과가 없어요.',
        week: '최근 7일 안에는 비교 결과가 없어요.',
        month: '최근 30일 안에는 비교 결과가 없어요.',
      },
      window: { day: '일간', week: '주간', month: '월간' },
      windowSpan: { day: '최근 24시간', week: '최근 7일', month: '최근 30일' },
      tally: '{{label}} {{count}}',
      windowAria: '리더보드 기간',
      scope: { me: '나', all: '전체' },
      scopeAria: '리더보드 범위',
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
      emptyIn: {
        day: 'No comparison results in the last 24 hours.',
        week: 'No comparison results in the last 7 days.',
        month: 'No comparison results in the last 30 days.',
      },
      window: { day: 'Daily', week: 'Weekly', month: 'Monthly' },
      windowSpan: { day: 'Last 24 hours', week: 'Last 7 days', month: 'Last 30 days' },
      tally: '{{label}} {{count}}',
      windowAria: 'Leaderboard period',
      scope: { me: 'Me', all: 'Everyone' },
      scopeAria: 'Leaderboard scope',
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
