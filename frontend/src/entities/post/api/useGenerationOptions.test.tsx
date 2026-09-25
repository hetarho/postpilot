import { create } from '@bufbuild/protobuf'
import { act, renderHook } from '@testing-library/react'
import { expect, it } from 'vitest'
import { GetPostResponseSchema, ListPostsResponseSchema, ProtoBlogField } from '@/shared/api'
import { createFakePostsTransport, type FakeOptionsSave } from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'
import { useGenerationOptions } from './useGenerationOptions'

function setup() {
  const optionSaves: FakeOptionsSave[] = []
  const transport = createFakePostsTransport({
    posts: [{ slug: 'post', targetLength: 1500, tagCount: 7, useMemory: true, field: 'cafe' }],
    optionSaves,
  })
  const queryClient = createTestQueryClient()
  const { result } = renderHook(() => useGenerationOptions(), {
    wrapper: withProviders(transport, queryClient),
  })
  return { optionSaves, transport, queryClient, result }
}

// POST-89: one 저장 is one request, and every member of it names the next value.
it('sends the whole set in one request and marks the post and the list stale', async () => {
  const { optionSaves, transport, queryClient, result } = setup()
  const postKey = getPostQueryKey(transport, 'post')
  const listKey = listPostsQueryKey(transport)
  queryClient.setQueryData(postKey, create(GetPostResponseSchema, {}))
  queryClient.setQueryData(listKey, create(ListPostsResponseSchema, {}))

  let saved: Awaited<ReturnType<typeof result.current.save>> | undefined
  await act(async () => {
    saved = await result.current.save('post', {
      targetLength: 1200,
      tagCount: 5,
      useMemory: false,
      qualityRules: ['title_saturation', 'composition'],
      field: 'restaurant',
    })
  })
  expect(optionSaves).toEqual([
    {
      slug: 'post',
      targetLength: 1200,
      tagCount: 5,
      useMemory: false,
      qualityRules: ['title_saturation', 'composition'],
      field: 'restaurant',
    },
  ])
  expect(saved?.post).toMatchObject({
    targetLength: 1200,
    tagCount: 5,
    useMemory: false,
    field: ProtoBlogField.RESTAURANT,
  })
  expect(queryClient.getQueryState(postKey)?.isInvalidated).toBe(true)
  expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
})

it('sends natural length as an absent target length and 없음 as a present UNSPECIFIED', async () => {
  const { optionSaves, result } = setup()

  let saved: Awaited<ReturnType<typeof result.current.save>> | undefined
  await act(async () => {
    saved = await result.current.save('post', {
      tagCount: 7,
      useMemory: true,
      qualityRules: [],
      field: '',
    })
  })
  expect(optionSaves).toEqual([
    {
      slug: 'post',
      targetLength: undefined,
      tagCount: 7,
      useMemory: true,
      qualityRules: [],
      field: '',
    },
  ])
  // Answered, not refused: the fake refuses a set missing any member.
  expect(saved?.post?.targetLength).toBeUndefined()
  expect(saved?.post?.field).toBe(ProtoBlogField.UNSPECIFIED)
  expect(saved?.post?.qualityRules).toEqual([])
})
