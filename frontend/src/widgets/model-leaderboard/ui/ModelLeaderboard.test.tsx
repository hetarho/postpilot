import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import type { LeaderboardEntry } from '@/entities/model-experiment'
import { ModelLeaderboard } from './ModelLeaderboard'

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
  }
  render(<ModelLeaderboard entries={[entry]} />)
  expect(screen.getByText('#1')).toBeInTheDocument()
  expect(screen.getByText('데이터 수집 중')).toBeInTheDocument()
  expect(screen.getByText('등록 해제')).toBeInTheDocument()
  expect(screen.getByText(/비용 미제공/)).toBeInTheDocument()
  expect(screen.queryByText('$0.000000')).not.toBeInTheDocument()
})
