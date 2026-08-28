import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { AUTOSAVE_DEBOUNCE_MS } from '@/shared/config'
import { type FakePostsOptions, createFakePostsTransport } from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import { discardDraftQueues } from './draft-queue'
import { useAutosave } from './useAutosave'

interface Typed {
  title: string
  memo: string
}

function setup(
  post: { slug: string; title: string; memo: string } | undefined,
  backend: FakePostsOptions = {},
) {
  const calls: string[] = []
  const transport = createFakePostsTransport({ calls, ...backend })
  const view = renderHook(
    ({ title, memo }: Typed) => useAutosave({ post, title, memo }),
    {
      wrapper: withProviders(transport, createTestQueryClient()),
      initialProps: { title: post?.title ?? '', memo: post?.memo ?? '' },
    },
  )
  return { ...view, saves: () => calls.filter((call) => call === 'SavePostDraft') }
}

/** Runs the timers and lets the resulting requests settle. */
async function tick(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

const EXISTING = { slug: '20260820-jeju', title: '제주', memo: '첫날' }

/** jsdom reports the document as visible and offers no way to background it, so the flag
 *  the handler reads has to be replaced before the event means anything. */
function hidePage() {
  Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true })
  document.dispatchEvent(new Event('visibilitychange'))
}

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true })
  discardDraftQueues()
  vi.useRealTimers()
})

describe('useAutosave', () => {
  // Opening a post must not write to it, and opening /posts/new must not create an empty
  // post just by being looked at.
  it('saves nothing while the text is untouched', async () => {
    const { saves, result } = setup(EXISTING)

    await tick(10_000)

    expect(saves()).toHaveLength(0)
    expect(result.current.state).toBe('idle')
  })

  it('saves a beat after the typing stops', async () => {
    const { rerender, saves, result } = setup(EXISTING)

    act(() => rerender({ title: '제주 3일', memo: '첫날' }))
    expect(result.current.state).toBe('dirty')
    await tick(AUTOSAVE_DEBOUNCE_MS)

    expect(saves()).toHaveLength(1)
    expect(result.current.state).toBe('saved')
  })

  // A1: the promise is that the text survives the tab, so every way out flushes.
  it.each([
    ['the page is hidden', hidePage],
    ['the page is unloading', () => window.dispatchEvent(new Event('pagehide'))],
  ])('flushes the pending save when %s', async (_name, leave) => {
    const { rerender, saves } = setup(EXISTING)

    act(() => rerender({ title: '제주 3일', memo: '첫날' }))
    await act(async () => {
      leave()
      await vi.advanceTimersByTimeAsync(0)
    })

    expect(saves()).toHaveLength(1)
  })

  it('flushes the pending save when the editor unmounts', async () => {
    const { rerender, unmount, saves } = setup(EXISTING)

    act(() => rerender({ title: '제주 3일', memo: '첫날' }))
    await act(async () => {
      unmount()
      await vi.advanceTimersByTimeAsync(0)
    })

    expect(saves()).toHaveLength(1)
  })

  // A 200 carrying no post is not a confirmation. Trusting it would mark the text saved
  // and, for a draft with no slug yet, leave the next edit creating a second post.
  it('treats a response without a post as a failed save', async () => {
    const { rerender, result } = setup(EXISTING, { saveReturnsNoPost: true })

    act(() => rerender({ title: '제주 3일', memo: '첫날' }))
    await tick(AUTOSAVE_DEBOUNCE_MS)

    expect(result.current.state).toBe('error')
  })
})
