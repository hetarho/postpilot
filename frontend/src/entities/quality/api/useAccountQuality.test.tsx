import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import { createFakeQualityTransport } from '@/test/quality'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useAccountQuality, usePrefetchAccountQuality } from './useAccountQuality'

const settle = () => new Promise((resolve) => setTimeout(resolve, 30))

it('reads nothing without an account or a post', async () => {
  for (const [ownerId, slug] of [
    ['', 'post'],
    ['alice', ''],
  ]) {
    const calls: string[] = []
    const transport = createFakeQualityTransport({ calls })
    const { result, unmount } = renderHook(() => useAccountQuality(ownerId, slug, 'ko'), {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    await settle()
    expect(calls).toEqual([])
    expect(result.current.quality).toBeUndefined()
    unmount()
  }
})

it('reads the aggregate for a post, all four metrics', async () => {
  const calls: string[] = []
  const transport = createFakeQualityTransport({ calls })
  const { result } = renderHook(() => useAccountQuality('alice', 'post', 'ko'), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await waitFor(() => expect(result.current.quality?.readings).toHaveLength(4))
  expect(calls).toEqual(['GetAccountQuality'])
})

it('prefetches for a post and for nothing else', async () => {
  const calls: string[] = []
  const transport = createFakeQualityTransport({ calls })
  const queryClient = createTestQueryClient()
  const { rerender } = renderHook(
    ({ slug }: { slug: string }) => usePrefetchAccountQuality('alice', slug, 'ko'),
    { wrapper: withProviders(transport, queryClient), initialProps: { slug: '' } },
  )
  await settle()
  expect(calls).toEqual([])

  rerender({ slug: 'post' })
  await waitFor(() => expect(calls).toEqual(['GetAccountQuality']))
  expect(
    queryClient.getQueryData(['quality', transport, 'alice', 'account', 'post', 'ko']),
  ).toBeDefined()
})
