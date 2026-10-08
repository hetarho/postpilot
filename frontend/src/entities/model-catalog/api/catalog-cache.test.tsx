import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { createConnectQueryKey } from '@connectrpc/connect-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it } from 'vitest'
import { ModelCatalogService, ProviderService, Stage } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { MODEL_PURPOSES } from '../config'
import { browseQueryKey } from './catalog-cache'
import {
  getComparisonPairsQueryKey,
  getSelectionsQueryKey,
  listModelsQueryKey,
  listRecommendationSetsQueryKey,
} from './catalog-mappers'
import {
  useAdminCatalog,
  useRefreshCatalog,
  useSetModelPurpose,
  useUpdateModel,
} from './useAdminCatalog'
import { useApplyCatalogDocument } from './useCatalogDocument'

afterEach(cleanup)

function seedViews(transport: ReturnType<typeof createRouterTransport>) {
  const cache = createTestQueryClient()
  cache.setDefaultOptions({
    queries: { ...cache.getDefaultOptions().queries, staleTime: 60_000 },
  })
  const exportKey = createConnectQueryKey({
    schema: ModelCatalogService.method.exportCatalogDocument,
    input: {},
    transport,
    cardinality: 'finite',
  })
  const browseKeys = MODEL_PURPOSES.map((purpose) => browseQueryKey(transport, purpose))
  for (const key of browseKeys)
    cache.setQueryData(key, create(ModelCatalogService.method.listCatalog.output))
  cache.setQueryData(exportKey, create(ModelCatalogService.method.exportCatalogDocument.output))
  cache.setQueryData(
    listModelsQueryKey(transport),
    create(ProviderService.method.listModels.output),
  )
  cache.setQueryData(
    getSelectionsQueryKey(transport),
    create(ProviderService.method.getSelections.output, {
      selections: [{ stage: Stage.WRITE, ref: { providerId: 'p', modelId: 'owner-choice' } }],
    }),
  )
  cache.setQueryData(
    getComparisonPairsQueryKey(transport),
    create(ProviderService.method.getComparisonPairs.output),
  )
  cache.setQueryData(
    listRecommendationSetsQueryKey(transport),
    create(ProviderService.method.listRecommendationSets.output),
  )
  return {
    cache,
    keys: [
      ...browseKeys,
      exportKey,
      listModelsQueryKey(transport),
      getSelectionsQueryKey(transport),
      getComparisonPairsQueryKey(transport),
      listRecommendationSetsQueryKey(transport),
    ],
  }
}

it.each(['bulk', 'registration', 'properties'] as const)(
  '%s curation invalidates every dependent view without overwriting the owner selection',
  async (action) => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(ModelCatalogService.method.applyCatalogDocument, () => ({ applied: true }))
      rpc(ModelCatalogService.method.setModelPurpose, () => ({}))
      rpc(ModelCatalogService.method.updateModel, () => ({}))
    })
    const { cache, keys } = seedViews(transport)
    const selection = cache.getQueryData(getSelectionsQueryKey(transport))
    function Curate() {
      const bulk = useApplyCatalogDocument()
      const registration = useSetModelPurpose()
      const properties = useUpdateModel()
      const completed = bulk.isSuccess || registration.isSuccess || properties.isSuccess
      return (
        <button
          onClick={() => {
            if (action === 'bulk') bulk.apply('# postpilot models v1\n[writing]\n')
            if (action === 'registration') registration.setPurpose('writer', 'writing', false)
            if (action === 'properties') properties.update('writer', 'writing', { level: 'value' })
          }}
        >
          {completed ? 'Complete' : 'Curate'}
        </button>
      )
    }
    render(<Curate />, { wrapper: withProviders(transport, cache) })
    await userEvent.setup().click(screen.getByRole('button', { name: 'Curate' }))
    await screen.findByRole('button', { name: 'Complete' })
    for (const key of keys) expect(cache.getQueryState(key)?.isInvalidated).toBe(true)
    expect(cache.getQueryData(getSelectionsQueryKey(transport))).toBe(selection)
  },
)

it('refreshes dependent views while reusing the returned active-purpose response', async () => {
  const calls: { purpose: string; refresh: boolean }[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ModelCatalogService.method.listCatalog, (request) => {
      calls.push({ purpose: request.purpose, refresh: request.refresh })
      return { fetchedAt: request.refresh ? 'fresh-writing' : 'fresh-other' }
    })
  })
  const { cache, keys } = seedViews(transport)
  function Browse() {
    const writing = useAdminCatalog('writing')
    useAdminCatalog('photo-analysis')
    const refresh = useRefreshCatalog('writing')
    return <button onClick={refresh.refresh}>{writing.catalog.fetchedAt || 'Refresh'}</button>
  }
  render(<Browse />, { wrapper: withProviders(transport, cache) })
  expect(calls).toEqual([])
  await userEvent.setup().click(screen.getByRole('button', { name: 'Refresh' }))
  await screen.findByRole('button', { name: 'fresh-writing' })
  await waitFor(() =>
    expect(calls).toEqual([
      { purpose: 'writing', refresh: true },
      { purpose: 'photo-analysis', refresh: false },
    ]),
  )
  expect(cache.getQueryState(browseQueryKey(transport, 'writing'))?.isInvalidated).toBe(false)
  for (const purpose of MODEL_PURPOSES) {
    expect(cache.getQueryState(browseQueryKey(transport, purpose))?.isInvalidated).toBe(
      purpose !== 'writing' && purpose !== 'photo-analysis',
    )
  }
  for (const key of keys.slice(MODEL_PURPOSES.length)) {
    expect(cache.getQueryState(key)?.isInvalidated).toBe(true)
  }
})
