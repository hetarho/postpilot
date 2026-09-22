import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import type { LeaderboardEntry } from '@/entities/model-experiment'
import { ModelLeaderboard } from './ModelLeaderboard'

/** One row as the server hands it over, so a test about tallies does not restate a dozen
 *  accounting fields it does not care about. */
function entryFixture(): LeaderboardEntry {
  return {
    rank: 1,
    model: { providerId: 'p', modelId: 'a' },
    modelLabel: 'A 모델',
    rating: 1516,
    matches: 4,
    wins: 3,
    losses: 1,
    winRate: 0.75,
    successfulCalls: 8,
    averageLatencyMs: 900n,
    promptTokens: 10n,
    completionTokens: 20n,
    totalCostMicrousd: 0n,
    costQuality: 'unavailable',
    provisional: false,
    active: false,
    recommended: false,
    disappeared: false,
    badgeTallies: [],
  }
}

it('renders server rank and badges without treating unavailable cost as zero', () => {
  const entry: LeaderboardEntry = {
    rank: 1,
    model: { providerId: 'p', modelId: 'gone' },
    modelLabel: '과거 모델',
    rating: 1516,
    matches: 1,
    wins: 1,
    losses: 0,
    winRate: 1,
    successfulCalls: 1,
    averageLatencyMs: 200n,
    promptTokens: 10n,
    completionTokens: 2n,
    totalCostMicrousd: 0n,
    costQuality: 'unavailable',
    provisional: true,
    active: false,
    recommended: false,
    disappeared: true,
    badgeTallies: [],
  }
  render(<ModelLeaderboard entries={[entry]} window="week" />)
  expect(screen.getByText('#1')).toBeInTheDocument()
  expect(screen.getByText('데이터 수집 중')).toBeInTheDocument()
  expect(screen.getByText('등록 해제')).toBeInTheDocument()
  expect(screen.getByText(/비용 미제공/)).toBeInTheDocument()
  expect(screen.queryByText('$0.000000')).not.toBeInTheDocument()
})

// An empty board is not the same statement in every window: told which period produced
// nothing, the reader can ask for a longer one instead of concluding they have no history.
it.each([
  ['day', '최근 24시간 안에는 비교 결과가 없어요.'],
  ['week', '최근 7일 안에는 비교 결과가 없어요.'],
  ['month', '최근 30일 안에는 비교 결과가 없어요.'],
] as const)('names the period that produced nothing (%s)', (window, message) => {
  render(<ModelLeaderboard entries={[]} window={window} />)
  expect(screen.getByText(message)).toBeInTheDocument()
})

// A row reads what its model's verdicts said about it in the same span the rating covers.
// Three of each at most: the board's own rank and rating must stay the first thing read.
it('states a row its badge tallies, most often first and bounded per group', () => {
  render(
    <ModelLeaderboard
      window="week"
      entries={[
        {
          ...entryFixture(),
          badgeTallies: [
            { badge: 'fast', count: 7 },
            { badge: 'natural', count: 4 },
            { badge: 'accurate', count: 2 },
            { badge: 'concise', count: 1 },
            { badge: 'slow', count: 3 },
          ],
        },
      ]}
    />,
  )
  expect(screen.getByText('속도가 빨라요 7')).toBeInTheDocument()
  expect(screen.getByText('자연스러워요 4')).toBeInTheDocument()
  expect(screen.getByText('내용이 정확해요 2')).toBeInTheDocument()
  expect(screen.getByText('느려요 3')).toBeInTheDocument()
  // The fourth positive one is over the bound.
  expect(screen.queryByText('간결해요 1')).not.toBeInTheDocument()
})

it('shows no tally row when a model earned none', () => {
  render(<ModelLeaderboard window="week" entries={[{ ...entryFixture(), badgeTallies: [] }]} />)
  expect(screen.queryByText(/속도가 빨라요/)).not.toBeInTheDocument()
})
