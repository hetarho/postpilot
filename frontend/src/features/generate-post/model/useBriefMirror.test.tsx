import { act, renderHook } from '@testing-library/react'
import { expect, it } from 'vitest'
import { POST_TAG_COUNT_DEFAULT } from '@/entities/post'
import { useBriefMirror } from './useBriefMirror'

it('keeps the owner’s edit until the server reports a different value', () => {
  const view = renderHook(({ post }) => useBriefMirror(post), {
    initialProps: {
      post: { targetLength: 800, tagCount: 5 } as { targetLength?: number; tagCount?: number },
    },
  })
  expect(view.result.current.targetLength).toBe(800)
  expect(view.result.current.tagCount).toBe(5)
  act(() => view.result.current.setTargetLength(1200))
  expect(view.result.current.targetLength).toBe(1200)
  // The same post again is not a change: a re-render must not throw the edit away.
  view.rerender({ post: { targetLength: 800, tagCount: 5 } })
  expect(view.result.current.targetLength).toBe(1200)
  // A save landed: the server's value is the truth again, in the same paint.
  view.rerender({ post: { targetLength: 1200, tagCount: 5 } })
  expect(view.result.current.targetLength).toBe(1200)
  view.rerender({ post: { targetLength: 400, tagCount: 5 } })
  expect(view.result.current.targetLength).toBe(400)
})

it('falls back to the product default for a tag count the post has not got, and mirrors the rest', () => {
  const view = renderHook(({ post }) => useBriefMirror(post), {
    initialProps: { post: undefined as { targetLength?: number; tagCount?: number } | undefined },
  })
  expect(view.result.current.tagCount).toBe(POST_TAG_COUNT_DEFAULT)
  expect(view.result.current.targetLength).toBeUndefined()
  view.rerender({ post: { tagCount: 3 } })
  expect(view.result.current.tagCount).toBe(3)
  view.rerender({ post: {} })
  expect(view.result.current.tagCount).toBe(POST_TAG_COUNT_DEFAULT)
})
