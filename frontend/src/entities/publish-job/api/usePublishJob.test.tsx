import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import { GetPublishJobResponseSchema, PublishingService } from '@/shared/api'
import { withProviders } from '@/test/session'
import { usePublishJob } from './usePublishJob'

it('normalizes an empty successful response without putting React Query into an error state', async () => {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PublishingService.method.getPublishJob, () => create(GetPublishJobResponseSchema))
  })
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

  const view = renderHook(() => usePublishJob('alice', 'post'), {
    wrapper: withProviders(transport, queryClient),
  })

  await waitFor(() => expect(view.result.current.isPending).toBe(false))
  expect(view.result.current.isError).toBe(false)
  expect(view.result.current.job).toBeUndefined()
})
