import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { AUTOSAVE_DEBOUNCE_MS } from '@/shared/config'
import { type FakeDraftSave, type FakePostsOptions, createFakePostsTransport } from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import { discardDraftQueues } from './draft-queue'
import { useAutosave } from './useAutosave'

interface Typed {
  title: string
  memo: string
}

function setup(
  post: { slug: string; title: string; memo: string; voice: { id: string } } | undefined,
  backend: FakePostsOptions = {},
) {
  const calls: string[] = []
  const draftSaves: FakeDraftSave[] = []
  const transport = createFakePostsTransport({ calls, draftSaves, ...backend })
  const view = renderHook(
    ({ title, memo }: Typed) =>
      // These cases are about the text pipeline, so every post here is 없음. The 용도 half of
      // the queue has its own file.
      useAutosave({
        post: post && { ...post, purpose: { id: '' } },
        title,
        memo,
        voiceId: post?.voice.id ?? 'voice-default',
        purposeId: '',
      }),
    {
      wrapper: withProviders(transport, createTestQueryClient()),
      initialProps: { title: post?.title ?? '', memo: post?.memo ?? '' },
    },
  )
  return {
    ...view,
    draftSaves,
    saves: () => calls.filter((call) => call === 'SavePostDraft'),
  }
}

/** Runs the timers and lets the resulting requests settle. */
async function tick(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}

const EXISTING = {
  slug: '20260820-jeju',
  title: '제주',
  memo: '첫날',
  voice: { id: 'voice-default', name: '기본 말투' },
}

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

  it('reassigns through the queue and leaves later text saves without a voice', async () => {
    const { result, rerender, draftSaves } = setup(EXISTING, {
      posts: [EXISTING],
      voices: [
        { id: 'voice-default', name: '기본 말투' },
        { id: 'voice-review', name: '리뷰' },
      ],
    })

    await act(() => result.current.reassign('voice-review'))
    expect(draftSaves).toEqual([{ slug: EXISTING.slug, voiceId: 'voice-review' }])

    act(() => rerender({ title: '제주 3일', memo: '첫날' }))
    await tick(AUTOSAVE_DEBOUNCE_MS)
    expect(draftSaves[1]).toEqual({ slug: EXISTING.slug, voiceId: undefined })
  })

  it('reports a refused reassignment to the caller', async () => {
    const { result } = setup(EXISTING, {
      posts: [{ ...EXISTING, activeJob: { id: 'job-1', status: 'running' } }],
      voices: [
        { id: 'voice-default', name: '기본 말투' },
        { id: 'voice-review', name: '리뷰' },
      ],
    })

    await act(async () => {
      await expect(result.current.reassign('voice-review')).rejects.toThrow()
    })
    // Taken back, with nothing left to retry: the queue is quiet, not stuck on a failure.
    expect(result.current.state).toBe('idle')
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
