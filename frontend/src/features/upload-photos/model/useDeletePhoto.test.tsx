import { act, renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import { usePost } from '@/entities/post'
import { createFakePostsTransport } from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useDeletePhoto } from './useDeletePhoto'
import { useRotatePhoto } from './useRotatePhoto'
import { useReorderPhotos } from './useReorderPhotos'

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

// Only the photo being turned is held, so another can be turned while it is in flight: each
// turn's answer reaches the cached post, not only the latest one's.
it('keeps both turns when a second photo is turned while the first is in flight', async () => {
  const slug = '20260820-jeju'
  let releaseFirst!: () => void
  const firstHeld = new Promise<void>((resolve) => (releaseFirst = resolve))
  const transport = createFakePostsTransport({
    posts: [
      {
        slug,
        images: [
          { id: 'img-1', filename: 'IMG_1.jpg', rotation: 0 },
          { id: 'img-2', filename: 'IMG_2.jpg', rotation: 0 },
        ],
      },
    ],
    rotateGate: (imageId) => (imageId === 'img-1' ? firstHeld : Promise.resolve()),
  })
  const { result } = renderHook(() => ({ read: usePost(slug), strip: useRotatePhoto(slug) }), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  await waitFor(() => expect(result.current.read.post?.images).toHaveLength(2))
  const [first, second] = result.current.read.post!.images
  act(() => result.current.strip.rotatePhoto(first!))
  act(() => result.current.strip.rotatePhoto(second!))
  await waitFor(() => expect(result.current.read.post?.images[1]?.rotation).toBe(90))
  releaseFirst()
  await waitFor(() => expect(result.current.read.post?.images[0]?.rotation).toBe(90))
  expect(result.current.read.post?.images.map((image) => image.rotation)).toEqual([90, 90])
})

// POST-116: a move puts the post's photos in the new order through ReorderImages, and the cached
// post takes that order at once, so every surface follows it without a refetch.
it('moves the last photo to the front through ReorderImages', async () => {
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
          { id: 'img-3', filename: 'IMG_3.jpg' },
        ],
      },
    ],
  })
  const { result } = renderHook(
    () => {
      const read = usePost(slug)
      return { read, strip: useReorderPhotos(slug, read.post?.images ?? []) }
    },
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  await waitFor(() => expect(result.current.read.post?.images).toHaveLength(3))

  act(() => result.current.strip.movePhoto(2, 0))

  await waitFor(() =>
    expect(result.current.read.post?.images.map((image) => image.id)).toEqual([
      'img-3',
      'img-1',
      'img-2',
    ]),
  )
  await waitFor(() => expect(calls).toContain('ReorderImages'))
  expect(result.current.strip.failure).toBeUndefined()
})

// A photo added or deleted since the post was read makes the sent order stale: the refusal reads
// the post again, which drops the local order for the server's.
it('reads the post again when the server calls the order stale', async () => {
  const slug = '20260820-jeju'
  const transport = createFakePostsTransport({
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
  const { result } = renderHook(
    () => {
      const read = usePost(slug)
      // A stale view: a third photo the server has never held.
      const images = read.post
        ? [...read.post.images, { ...read.post.images[0]!, id: 'gone', filename: 'GONE.jpg' }]
        : []
      return { read, strip: useReorderPhotos(slug, images) }
    },
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  await waitFor(() => expect(result.current.read.post?.images).toHaveLength(2))

  act(() => result.current.strip.movePhoto(1, 0))

  await waitFor(() => expect(result.current.strip.failure?.reason).toBe('POST_PHOTO_ORDER_STALE'))
  await waitFor(() =>
    expect(result.current.read.post?.images.map((image) => image.id)).toEqual(['img-1', 'img-2']),
  )
})
