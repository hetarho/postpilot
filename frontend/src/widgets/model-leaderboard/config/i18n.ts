import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    legacyLeaderboard: {
      description:
        '실제 글을 나란히 읽고 고르는 글쓰기 테스트로 비교해 보세요. 이전 유료 결과는 테스트 기록에서 볼 수 있어요.',
      openTests: '글쓰기 테스트 시작하기',
      openHistory: '글쓰기 테스트 기록',
    },
    unavailable: '등록 해제된 모델',
    leaderboard: {
      empty: '아직 순위를 매긴 비교 결과가 없어요.',
      emptyIn: {
        day: '최근 24시간 안에는 순위를 매긴 비교가 없어요.',
        week: '최근 7일 안에는 순위를 매긴 비교가 없어요.',
        month: '최근 30일 안에는 순위를 매긴 비교가 없어요.',
      },
      window: { day: '일간', week: '주간', month: '월간' },
      windowSpan: { day: '최근 24시간', week: '최근 7일', month: '최근 30일' },
      tally: '{{label}} {{count}}',
      windowAria: '리더보드 기간',
      scope: { me: '나', all: '전체' },
      scopeAria: '리더보드 범위',
      collecting: '평가 3회 미만',
      active: '활성',
      recommended: '추천',
      disappeared: '등록 해제',
      record: '{{evaluations}}회 평가 · 상대별 {{matches}}전 {{wins}}승 {{losses}}패 {{draws}}무',
      metrics: '성공 호출 {{calls}} · 평균 {{latency}}ms · 토큰 {{prompt}} / {{completion}}',
    },
  },
  en: {
    legacyLeaderboard: {
      description:
        'Compare complete posts in writing tests and choose the result you prefer. Earlier paid results remain in test history.',
      openTests: 'Start a writing test',
      openHistory: 'Writing test history',
    },
    unavailable: 'Unregistered model',
    leaderboard: {
      empty: 'No ranked comparisons yet.',
      emptyIn: {
        day: 'No ranked comparisons in the last 24 hours.',
        week: 'No ranked comparisons in the last 7 days.',
        month: 'No ranked comparisons in the last 30 days.',
      },
      window: { day: 'Daily', week: 'Weekly', month: 'Monthly' },
      windowSpan: { day: 'Last 24 hours', week: 'Last 7 days', month: 'Last 30 days' },
      tally: '{{label}} {{count}}',
      windowAria: 'Leaderboard period',
      scope: { me: 'Me', all: 'Everyone' },
      scopeAria: 'Leaderboard scope',
      collecting: 'Fewer than 3 evaluations',
      active: 'Active',
      recommended: 'Recommended',
      disappeared: 'Unregistered',
      record:
        'Evaluated comparisons {{evaluations}} · Pairwise outcomes {{matches}}: Wins {{wins}}, Losses {{losses}}, Draws {{draws}}',
      metrics:
        '{{calls}} successful calls · {{latency}}ms average · tokens {{prompt}} / {{completion}}',
    },
  },
} as const satisfies I18nFragment
