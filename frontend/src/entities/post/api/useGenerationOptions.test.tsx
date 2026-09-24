import { create } from '@bufbuild/protobuf'
import { act, renderHook } from '@testing-library/react'
import { expect, it } from 'vitest'
import { GetPostResponseSchema, ListPostsResponseSchema } from '@/shared/api'
import { createFakePostsTransport, type FakePostsOptions } from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import { getPostQueryKey, listPostsQueryKey } from './post-queries'
import { useGenerationOptions } from './useGenerationOptions'

// POST-81: the tick save sends the whole set and resends the length, which is the one member the
// server does not read by presence; the tag count and the memory flag stay absent, which keeps them.
it('saves the whole tick set with the length and nothing else', async () => {
  const qualityRuleSaves: NonNullable<FakePostsOptions['qualityRuleSaves']> = []
  const generationOptionSaves: Array<number | undefined> = []
  const tagCountSaves: Array<number | undefined> = []
  const memoryOptionSaves: Array<boolean | undefined> = []
  const transport = createFakePostsTransport({
    posts: [{ slug: 'post', targetLength: 1500, tagCount: 7, useMemory: true }],
    qualityRuleSaves,
    generationOptionSaves,
    tagCountSaves,
    memoryOptionSaves,
  })
  const queryClient = createTestQueryClient()
  const postKey = getPostQueryKey(transport, 'post')
  const listKey = listPostsQueryKey(transport)
  queryClient.setQueryData(postKey, create(GetPostResponseSchema, {}))
  queryClient.setQueryData(listKey, create(ListPostsResponseSchema, {}))
  const { result } = renderHook(() => useGenerationOptions(), {
    wrapper: withProviders(transport, queryClient),
  })

  let saved: Awaited<ReturnType<typeof result.current.saveQualityRules>> | undefined
  await act(async () => {
    saved = await result.current.saveQualityRules('post', ['title_saturation', 'composition'], 1500)
  })
  expect(qualityRuleSaves).toEqual([['title_saturation', 'composition']])
  expect(generationOptionSaves).toEqual([1500])
  expect(tagCountSaves).toEqual([undefined])
  expect(memoryOptionSaves).toEqual([undefined])
  expect(saved?.post).toMatchObject({ targetLength: 1500, tagCount: 7, useMemory: true })
  // The post holds the ticks now, so its entry and the list are re-read.
  expect(queryClient.getQueryState(postKey)?.isInvalidated).toBe(true)
  expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)

  // An empty set is present and clears the ticks.
  await act(async () => {
    await result.current.saveQualityRules('post', [], undefined)
  })
  expect(qualityRuleSaves).toEqual([['title_saturation', 'composition'], []])
})
