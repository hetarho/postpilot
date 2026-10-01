import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
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
    draws: 0,
    evaluatedComparisons: 1,
    winRate: 0.75,
    successfulCalls: 8,
    averageLatencyMs: 900n,
    promptTokens: 10n,
    completionTokens: 20n,
    provisional: false,
    active: false,
    recommended: false,
    disappeared: false,
    badgeTallies: [],
  }
}

it('renders Elo and pairwise evidence without supplier cost', () => {
  const entry: LeaderboardEntry = {
    rank: 1,
    model: { providerId: 'p', modelId: 'gone' },
    modelLabel: '과거 모델',
    rating: 1516,
    matches: 1,
    wins: 1,
    losses: 0,
    draws: 0,
    evaluatedComparisons: 1,
    winRate: 1,
    successfulCalls: 1,
    averageLatencyMs: 200n,
    promptTokens: 10n,
    completionTokens: 2n,
    provisional: true,
    active: false,
    recommended: false,
    disappeared: true,
    badgeTallies: [],
  }
  render(<ModelLeaderboard entries={[entry]} window="week" />)
  expect(screen.getByText('#1')).toBeInTheDocument()
  expect(screen.getByText('평가 3회 미만')).toBeInTheDocument()
  expect(screen.getByText('등록 해제')).toBeInTheDocument()
  expect(screen.getByText('1회 평가 · 상대별 1전 1승 0패 0무')).toBeInTheDocument()
  expect(screen.queryByText(/비용 미제공/)).not.toBeInTheDocument()
  expect(screen.queryByText('$0.000000')).not.toBeInTheDocument()
})

// An empty board is not the same statement in every window: told which period produced
// nothing, the reader can ask for a longer one instead of concluding they have no history.
it.each([
  ['day', '최근 24시간 안에는 순위를 매긴 비교가 없어요.'],
  ['week', '최근 7일 안에는 순위를 매긴 비교가 없어요.'],
  ['month', '최근 30일 안에는 순위를 매긴 비교가 없어요.'],
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

// MODEL-62/63: `기타` is neither praise nor complaint. It is tallied apart from both groups,
// shown below them, never in the warning tone, and never takes a negative's place.
it('keeps 기타 apart from the negatives, below both groups, and never as a warning', () => {
  render(
    <ModelLeaderboard
      window="week"
      entries={[
        {
          ...entryFixture(),
          badgeTallies: [
            { badge: 'other', count: 9 },
            { badge: 'slow', count: 5 },
            { badge: 'ai_like', count: 4 },
            { badge: 'verbose', count: 3 },
            { badge: 'repetitive', count: 2 },
            { badge: 'fast', count: 1 },
          ],
        },
      ]}
    />,
  )
  const other = screen.getByText(/^기타 9$/)
  expect(screen.getByText('느려요 5')).toBeInTheDocument()
  expect(screen.getByText('AI 같아요 4')).toBeInTheDocument()
  expect(screen.getByText('장황해요 3')).toBeInTheDocument()
  // Three negatives, however often 기타 was given: it is not one of them.
  expect(screen.queryByText('반복이 많아요 2')).not.toBeInTheDocument()
  expect(other.className).not.toMatch(/warning/)
  const chips = Array.from(other.parentElement!.children).map((chip) => chip.textContent)
  expect(chips.at(-1)).toBe('기타 9')
  expect(chips[0]).toBe('속도가 빨라요 1')
})

it('shows no tally row when a model earned none', () => {
  render(<ModelLeaderboard window="week" entries={[{ ...entryFixture(), badgeTallies: [] }]} />)
  expect(screen.queryByText(/속도가 빨라요/)).not.toBeInTheDocument()
})

// MODEL-38: a dismissal of two delivered candidates is one loss each against the fixed
// reference, which the board never ranks. A model seen only in dismissals is a row of losses.
it('shows the losses a dismissal counts, with no reference opponent on the board', () => {
  render(
    <ModelLeaderboard
      window="week"
      entries={[
        {
          ...entryFixture(),
          rating: 1484,
          matches: 1,
          wins: 0,
          losses: 1,
          winRate: 0,
          provisional: true,
        },
      ]}
    />,
  )
  expect(screen.getByText('1회 평가 · 상대별 1전 0승 1패 0무')).toBeInTheDocument()
  expect(screen.getByText('Elo 1484')).toBeInTheDocument()
  expect(screen.getAllByRole('listitem')).toHaveLength(1)
})

it('shows usage without supplier cost', () => {
  render(<ModelLeaderboard entries={[entryFixture()]} window="week" />)
  expect(screen.getByText(/성공 호출 8 · 평균 900ms · 토큰 10 \/ 20/)).toBeInTheDocument()
  expect(screen.queryByText(/\$/)).not.toBeInTheDocument()
  expect(screen.queryByText(/비용 미제공/)).not.toBeInTheDocument()
})

it('counts one five-way ranking once and labels four pairwise outcomes including ties', () => {
  render(
    <ModelLeaderboard
      entries={[
        {
          ...entryFixture(),
          evaluatedComparisons: 1,
          matches: 4,
          wins: 2,
          losses: 1,
          draws: 1,
          provisional: true,
        },
      ]}
      window="week"
    />,
  )
  expect(screen.getByText('1회 평가 · 상대별 4전 2승 1패 1무')).toBeInTheDocument()
  expect(screen.getByText('평가 3회 미만')).toBeInTheDocument()
})

it('explains a provisional tie in English without supplier cost', () => {
  initializeI18n('en')
  try {
    render(
      <ModelLeaderboard
        entries={[
          {
            ...entryFixture(),
            matches: 4,
            wins: 2,
            losses: 1,
            draws: 1,
            evaluatedComparisons: 1,
            provisional: true,
          },
        ]}
        window="week"
      />,
    )
    expect(
      screen.getByText('Evaluated comparisons 1 · Pairwise outcomes 4: Wins 2, Losses 1, Draws 1'),
    ).toBeInTheDocument()
    expect(screen.getByText('Fewer than 3 evaluations')).toBeInTheDocument()
    expect(screen.queryByText(/\$/)).not.toBeInTheDocument()
  } finally {
    initializeI18n('ko')
  }
})
