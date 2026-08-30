import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import {
  ComparisonPairSchema,
  GetComparisonPairsResponseSchema,
  ListRecommendationSetsResponseSchema,
  ProviderService,
  SaveComparisonPairResponseSchema,
  SelectionSchema,
  SelectionSlot,
  Stage,
} from '@/shared/api'
import { withProviders } from '@/test/session'
import { useComparisonPairSavePending, useModelSetup, useSaveComparisonPair } from './useModelSetup'

const pair = (a: string, b: string) =>
  create(ComparisonPairSchema, {
    stage: Stage.WRITE,
    candidateA: create(SelectionSchema, {
      stage: Stage.WRITE,
      slot: SelectionSlot.CANDIDATE_A,
      ref: { providerId: 'openrouter', modelId: a },
    }),
    candidateB: create(SelectionSchema, {
      stage: Stage.WRITE,
      slot: SelectionSlot.CANDIDATE_B,
      ref: { providerId: 'openrouter', modelId: b },
    }),
  })

/** The window this closes: `useComparisonPairSavePending` goes false when the mutation resolves, so
 *  a start that reads the pair right then must already see the pair that was just saved — not the
 *  one the next refetch happens to be carrying. The refetch here never lands during the assertion,
 *  which is what makes the cache write, and not the invalidation, the thing under test. */
it('exposes the saved comparison pair as soon as the save stops being pending', async () => {
  let reads = 0
  const blockRefetch = new Promise<void>(() => {})
  let releaseSave!: () => void
  const saveGate = new Promise<void>((resolve) => {
    releaseSave = resolve
  })
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.getComparisonPairs, async () => {
      reads += 1
      if (reads > 1) await blockRefetch
      return create(GetComparisonPairsResponseSchema, { pairs: [pair('old-a', 'old-b')] })
    })
    rpc(ProviderService.method.listRecommendationSets, () =>
      create(ListRecommendationSetsResponseSchema, {}),
    )
    rpc(ProviderService.method.saveComparisonPair, async () => {
      await saveGate
      return create(SaveComparisonPairResponseSchema, { pair: pair('new-a', 'new-b') })
    })
  })
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

  const view = renderHook(
    () => ({
      pairs: useModelSetup().pairs,
      pending: useComparisonPairSavePending(),
      save: useSaveComparisonPair().save,
    }),
    { wrapper: withProviders(transport, queryClient) },
  )

  await waitFor(() => expect(view.result.current.pairs[0]?.candidateA?.ref.modelId).toBe('old-a'))

  // Deliberately not awaited: `save()` resolves only after the invalidation's refetch, and the
  // whole point is what a reader sees BEFORE that — the moment the pending flag drops.
  void view.result.current.save(
    'write',
    { providerId: 'openrouter', modelId: 'new-a' },
    { providerId: 'openrouter', modelId: 'new-b' },
  )

  await waitFor(() => expect(view.result.current.pending).toBe(true))
  releaseSave()
  await waitFor(() => expect(view.result.current.pending).toBe(false))
  expect(view.result.current.pairs[0]?.candidateA?.ref.modelId).toBe('new-a')
  expect(view.result.current.pairs[0]?.candidateB?.ref.modelId).toBe('new-b')
})
