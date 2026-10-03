import { act, renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import { usePost } from '@/entities/post'
import { createFakePostsTransport } from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useDeletePhoto } from './useDeletePhoto'
import { useRotatePhoto } from './useRotatePhoto'

// POST-39, browser half: a delete calls DeleteImage, and the cached post loses the
// photo so the strip drops it without a refetch.
it('deletes a photo through DeleteImage', async () => {
  const calls: string[] = []
  const slug = '20260820-jeju'
  const transport = createFakePostsTransport({
    calls,
    posts: [
      {
        slug,
        images: [
          { id: 'img-1', filename: 'IMG_1.jpg' },
          { id: 'img-2', filename: 'IMG_2.jpg' },
        ],
      },
    ],
  })
  const { result } = renderHook(() => ({ read: usePost(slug), strip: useDeletePhoto(slug) }), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await waitFor(() => expect(result.current.read.post?.images).toHaveLength(2))

  const first = result.current.read.post!.images[0]
  act(() => result.current.strip.deletePhoto(first))

  await waitFor(() =>
    expect(result.current.read.post?.images.map((image) => image.id)).toEqual(['img-2']),
  )
  expect(calls).toContain('DeleteImage')
  expect(result.current.strip.failedId).toBeUndefined()
})

// POST-107: the strip's 회전 turns a photo a quarter clockwise through RotateImage, and the cached
// post takes the turned photo so every surface shows it without a refetch.
it('turns a photo a quarter clockwise through RotateImage', async () => {
  const calls: string[] = []
  const slug = '20260820-jeju'
  const transport = createFakePostsTransport({
    calls,
    posts: [{ slug, images: [{ id: 'img-1', filename: 'IMG_1.jpg', rotation: 270 }] }],
  })
  const { result } = renderHook(() => ({ read: usePost(slug), strip: useRotatePhoto(slug) }), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await waitFor(() => expect(result.current.read.post?.images[0]?.rotation).toBe(270))
  act(() => result.current.strip.rotatePhoto(result.current.read.post!.images[0]!))
  await waitFor(() => expect(result.current.read.post?.images[0]?.rotation).toBe(0))
  expect(calls).toContain('RotateImage')
  act(() => result.current.strip.rotatePhoto(result.current.read.post!.images[0]!))
  await waitFor(() => expect(result.current.read.post?.images[0]?.rotation).toBe(90))
  expect(result.current.strip.failure).toBeUndefined()
})
