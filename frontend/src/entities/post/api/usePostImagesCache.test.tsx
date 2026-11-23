import { act, renderHook } from '@testing-library/react'
import { createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { expect, it, vi } from 'vitest'
import { GetPostResponseSchema, PostSchema, type GetPostResponse } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'
import { usePostImagesCache } from './usePostImagesCache'

it('invalidates lists on attachment changes while retaining detail URLs, but not on rotation or reorder', () => {
  const transport = createRouterTransport(() => {})
  const client = createTestQueryClient()
  const key = getPostQueryKey(transport, 'post')
  client.setQueryData(
    key,
    create(GetPostResponseSchema, { post: create(PostSchema, { slug: 'post' }) }),
  )
  const invalidate = vi.spyOn(client, 'invalidateQueries')
  const { result } = renderHook(() => usePostImagesCache(), {
    wrapper: withProviders(transport, client),
  })
  const photo = { id: 'p', filename: 'p.jpg', width: 1, height: 1, bytes: 1, viewUrl: 'blob:photo' }
  const video = {
    id: 'v',
    filename: 'v.mp4',
    width: 1,
    height: 1,
    bytes: 1,
    durationMs: 1,
    contentType: 'video/mp4',
    viewUrl: 'blob:video',
  }
  act(() => {
    result.current.append('post', photo)
    result.current.appendVideo('post', video)
  })
  expect(client.getQueryData<GetPostResponse>(key)?.post?.images[0]?.viewUrl).toBe('blob:photo')
  act(() => {
    result.current.replace('post', photo)
    result.current.reorder('post', ['p'])
  })
  expect(invalidate).toHaveBeenCalledTimes(2)
  act(() => {
    result.current.remove('post', 'p')
    result.current.removeVideo('post', 'v')
  })
  expect(invalidate).toHaveBeenCalledTimes(4)
  expect(invalidate).toHaveBeenCalledWith({ queryKey: listPostsQueryKey(transport) })
})
