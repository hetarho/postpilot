import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import type { LeaderboardEntry } from '@/entities/model-experiment'
import { ModelLeaderboard } from './ModelLeaderboard'

it.each([
  { entries: [] },
  { entries: [{ rank: 1, rating: 1600, modelLabel: 'Legacy ranked model' }] },
])(
  'replaces old rankings with common writing-test and retained-history entry points',
  ({ entries }) => {
    render(<ModelLeaderboard entries={entries as LeaderboardEntry[]} window="week" />)
    expect(screen.getByRole('link', { name: '글쓰기 테스트 시작하기' })).toHaveAttribute(
      'href',
      '/tests',
    )
    expect(screen.getByRole('link', { name: '글쓰기 테스트 기록' })).toHaveAttribute(
      'href',
      '/tests/history',
    )
    expect(screen.queryByText(/Elo|1600|Legacy ranked model/)).not.toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  },
)
