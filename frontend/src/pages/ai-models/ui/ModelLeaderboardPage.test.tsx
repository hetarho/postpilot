import { act, screen, waitFor, within } from '@testing-library/react'
import { beforeEach, expect, it } from 'vitest'
import { LeaderboardEntrySchema, ProtoPlan, Stage } from '@/shared/api'
import { create } from '@bufbuild/protobuf'
import type { FakeExperimentsOptions } from '@/test/experiments'
import { paidActions, renderLegacyModelRoute } from '@/test/legacy-model-routes'

beforeEach(() => sessionStorage.clear())

it('resolves master/global rank links to read-only paid history without Elo or supplier costs', async () => {
  const { router, procedures } = renderLegacyModelRoute(
    '/ai-models/leaderboard?scope=all&window=day&stage=write',
    {
      user: { id: 'root', plan: ProtoPlan.MASTER },
      experiments: {
        leaderboardEntries: [
          create(LeaderboardEntrySchema, {
            rank: 1,
            modelLabel: 'Model',
            rating: 1516,
            promptTokens: 10n,
            completionTokens: 20n,
          }),
        ],
        history: [{ id: 'paid', stage: Stage.WRITE, postSlug: 'source' }],
      },
    },
  )
  await screen.findByRole('heading', { name: '테스트 기록', level: 1 })
  expect(router.state.location.pathname).toBe('/tests/history')
  expect(router.state.location.search.stage).toBe('write')
  expect(router.state.location.search).not.toHaveProperty('scope')
  expect(router.state.location.search).not.toHaveProperty('window')
  const legacy = within(screen.getByRole('region', { name: '이전 유료 비교 기록' }))
  expect(await legacy.findByRole('link', { name: '계속 보기' })).toHaveAttribute(
    'href',
    expect.stringContaining('/paid?entry='),
  )
  expect(screen.queryByText(/Elo|1516|\$/)).toBeNull()
  expect(screen.queryByRole('tab', { name: '주간' })).toBeNull()
  expect(procedures).not.toContain('GetLeaderboard')
  expect(paidActions(procedures)).toEqual([])
})

it.each(['', '?window=all-time&scope=friends', '?stage=analyze&window=day'])(
  'drops retired rank filters at %s and reads owner history without analysis comparisons',
  async (search) => {
    const reads: NonNullable<FakeExperimentsOptions['reads']> = []
    const { router, procedures } = renderLegacyModelRoute(`/ai-models/leaderboard${search}`, {
      user: { id: 'alice' },
      experiments: { reads },
    })
    await screen.findByRole('heading', { name: '테스트 기록', level: 1 })
    expect(router.state.location.pathname).toBe('/tests/history')
    expect(router.state.location.search).not.toHaveProperty('window')
    expect(router.state.location.search).not.toHaveProperty('scope')
    expect(router.state.location.search.stage).toBeUndefined()
    await waitFor(() =>
      expect(reads).toContainEqual(
        expect.objectContaining({ kind: 'history', stage: Stage.UNSPECIFIED }),
      ),
    )
    expect(reads.some((read) => read.kind === 'leaderboard' || read.stage === Stage.ANALYZE)).toBe(
      false,
    )
    expect(paidActions(procedures)).toEqual([])
  },
)

it('retains canonical stage/source through navigation and browser Back instead of changing historical rank controls', async () => {
  const { router, procedures } = renderLegacyModelRoute(
    '/ai-models/leaderboard?stage=observe&source=source-a',
    { user: { id: 'alice' } },
  )
  await screen.findByRole('heading', { name: '테스트 기록', level: 1 })
  expect(router.state.location.search).toMatchObject({ stage: 'observe', source: 'source-a' })
  await act(() =>
    router.navigate({ to: '/tests/history', search: { stage: 'write', source: 'source-b' } }),
  )
  await waitFor(() =>
    expect(router.state.location.search).toMatchObject({ stage: 'write', source: 'source-b' }),
  )
  await act(() => router.history.back())
  await waitFor(() =>
    expect(router.state.location.search).toMatchObject({ stage: 'observe', source: 'source-a' }),
  )
  expect(procedures).not.toContain('GetLeaderboard')
  expect(paidActions(procedures)).toEqual([])
})

it('does not expose Alice private test history to another signed-in owner', async () => {
  const alice = renderLegacyModelRoute(
    '/ai-models/leaderboard',
    { user: { id: 'alice' } },
    { readableTest: true },
  )
  expect(await screen.findByRole('link', { name: '테스트 이어보기' })).toBeVisible()
  expect(alice.fixture.admissions).toEqual([])
  alice.unmount()
  const bob = renderLegacyModelRoute(
    '/ai-models/leaderboard',
    { user: { id: 'bob' } },
    { readableTest: true },
  )
  await screen.findByRole('heading', { name: '테스트 기록', level: 1 })
  await waitFor(() => expect(bob.procedures).toContain('ListWritingTests'))
  await waitFor(() => expect(screen.getAllByText('아직 테스트 기록이 없어요.')).toHaveLength(2))
  expect(screen.queryByRole('link', { name: '테스트 이어보기' })).toBeNull()
  expect(bob.fixture.votes).toEqual([])
  expect(paidActions(bob.procedures)).toEqual([])
})
