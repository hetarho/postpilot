import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { renderHook } from '@testing-library/react'
import { expect, it } from 'vitest'
import { appFailureFromConnect, PostContentSchema, PostService } from '@/shared/api'
import { connectAppError } from '@/test/app-error'
import { withProviders } from '@/test/session'
import { ContentRevisionConflictError, useSavePostContent } from './useSavePostContent'

function saveAgainst(cause: ConnectError) {
  const transport = createRouterTransport(({ rpc }) => {
    rpc(PostService.method.savePostContent, () => {
      throw cause
    })
  })
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const view = renderHook(() => useSavePostContent(), {
    wrapper: withProviders(transport, queryClient),
  })
  return view.result.current.save('post', create(PostContentSchema), 1n)
}

it('turns only POST_CONTENT_STALE into the local revision-conflict control state', async () => {
  await expect(
    saveAgainst(connectAppError('POST_CONTENT_STALE', Code.Aborted)),
  ).rejects.toBeInstanceOf(ContentRevisionConflictError)
})

it('keeps malformed Aborted failures generic instead of inferring a stale revision', async () => {
  const cause = await saveAgainst(new ConnectError('private transport prose', Code.Aborted)).catch(
    (error: unknown) => error,
  )

  expect(cause).not.toBeInstanceOf(ContentRevisionConflictError)
  expect(appFailureFromConnect(cause)).toEqual({ reason: 'UNKNOWN_FAILURE', params: {} })
})
