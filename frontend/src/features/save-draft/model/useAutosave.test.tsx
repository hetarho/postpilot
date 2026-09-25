import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { AUTOSAVE_DEBOUNCE_MS, AUTOSAVE_RETRY_BASE_MS } from '@/shared/config'
import { type FakeDraftSave, type FakePostsOptions, createFakePostsTransport } from '@/test/posts'
import { createTestQueryClient, withProviders } from '@/test/session'
import type { PostStatus } from '@/entities/post'
import { discardDraftQueues, type TemplateAnswerDraft } from './draft-queue'
import { useAutosave } from './useAutosave'

/** One identity for every render, as `DraftEditor`'s memoized patch has: these cases are about the
 *  text pipeline, and the data fields have their own file. */
const NO_ANSWERS: TemplateAnswerDraft[] = []
const NO_TEMPLATE = () => NO_ANSWERS

interface Row {
  slug: string
  title: string
  memo: string
  status: PostStatus
  voice: { id: string }
  targetLanguage: 'ko' | 'en'
}

/** The post as the hook is handed it: every post here is 없음, whose half of the queue has its own
 *  file. A fresh answers array each time, as `usePost` maps one per read. */
const opened = (row: Row) => ({ ...row, template: { id: '' }, templateAnswers: [] })

function setup(
  row: Row | undefined,
  backend: FakePostsOptions = {},
  initialTarget: 'ko' | 'en' = 'ko',
) {
  const calls: string[] = []
  const draftSaves: FakeDraftSave[] = []
  const transport = createFakePostsTransport({ calls, draftSaves, ...backend })
  const view = renderHook(
    ({ post }: { post: ReturnType<typeof opened> | undefined }) =>
      useAutosave({
        post,
        answerPatch: NO_TEMPLATE,
        voiceId: row?.voice.id ?? 'voice-default',
        templateId: '',
        targetLanguage: row?.targetLanguage ?? initialTarget,
      }),
    {
      wrapper: withProviders(transport, createTestQueryClient()),
      initialProps: { post: row && opened(row) },
    },
  )
  return {
    ...view,
    draftSaves,
    saves: () => calls.filter((call) => call === 'SavePostDraft'),
    /** A refetch: the same post read again, or with the status the server now reports. */
    refetch: (status: PostStatus = row?.status ?? 'draft') =>
      act(() => view.rerender({ post: row && opened({ ...row, status }) })),
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
  status: 'finalized' as const,
  voice: { id: 'voice-default', name: '기본 말투' },
  targetLanguage: 'ko' as const,
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
  it('sends a concrete locale-derived target on the first create request', async () => {
    const { result, draftSaves } = setup(undefined, {}, 'en')

    act(() => result.current.setTitle('First post'))
    await tick(AUTOSAVE_DEBOUNCE_MS)

    expect(draftSaves[0]).toMatchObject({
      slug: '',
      voiceId: 'voice-default',
      targetLanguage: 'en',
    })
  })

  // Opening a post must not write to it, and opening /posts/new must not create an empty
  // post just by being looked at.
  it('saves nothing while the text is untouched', async () => {
    const { saves, result } = setup(EXISTING)

    await tick(10_000)

    expect(saves()).toHaveLength(0)
    expect(result.current.state).toBe('idle')
  })

  it('saves a beat after the typing stops', async () => {
    const { saves, result } = setup(EXISTING)

    act(() => result.current.setTitle('제주 3일'))
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
    const { result, saves } = setup(EXISTING)

    act(() => result.current.setTitle('제주 3일'))
    await act(async () => {
      leave()
      await vi.advanceTimersByTimeAsync(0)
    })

    expect(saves()).toHaveLength(1)
  })

  it('flushes the pending save when the editor unmounts', async () => {
    const { result, unmount, saves } = setup(EXISTING)

    act(() => result.current.setTitle('제주 3일'))
    await act(async () => {
      unmount()
      await vi.advanceTimersByTimeAsync(0)
    })

    expect(saves()).toHaveLength(1)
  })

  it('reassigns through the queue and leaves later text saves without a voice', async () => {
    const { result, draftSaves } = setup(EXISTING, {
      posts: [EXISTING],
      voices: [
        { id: 'voice-default', name: '기본 말투' },
        { id: 'voice-review', name: '리뷰' },
      ],
    })

    await act(() => result.current.reassign('voice-review'))
    expect(draftSaves).toEqual([
      {
        slug: EXISTING.slug,
        voiceId: 'voice-review',
        templateId: undefined,
        templateAnswers: [],
        targetLanguage: undefined,
      },
    ])

    act(() => result.current.setTitle('제주 3일'))
    await tick(AUTOSAVE_DEBOUNCE_MS)
    expect(draftSaves[1]).toEqual({
      slug: EXISTING.slug,
      voiceId: undefined,
      templateId: undefined,
      templateAnswers: [],
      targetLanguage: undefined,
    })
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
    const { result } = setup(EXISTING, { saveReturnsNoPost: true })

    act(() => result.current.setTitle('제주 3일'))
    await tick(AUTOSAVE_DEBOUNCE_MS)

    expect(result.current.state).toBe('error')
  })

  // POST-86: a post published in another tab refuses every save the same way, so the text is
  // taken back rather than retried, and the line never reads 다시 시도 중 over it.
  it('takes a save refused as published back instead of retrying it', async () => {
    const { saves, result } = setup(EXISTING, {
      posts: [{ slug: EXISTING.slug, title: EXISTING.title, memo: EXISTING.memo }],
      publishOnDraftSave: EXISTING.slug,
    })

    act(() => result.current.setTitle('제주 3일'))
    await tick(AUTOSAVE_DEBOUNCE_MS)
    expect(saves()).toHaveLength(1)
    expect(result.current.state).toBe('idle')

    await tick(AUTOSAVE_RETRY_BASE_MS * 16)
    expect(saves()).toHaveLength(1)
    expect(result.current.state).toBe('idle')
  })
})

// POST-86, review F25: the autosave decides the lock itself, from the post it is handed.
describe('a published post', () => {
  it('sends nothing for a published post, across edits and refetches', async () => {
    const { result, saves, refetch } = setup({ ...EXISTING, status: 'published' })

    act(() => result.current.setTitle('제주 3일'))
    expect(result.current.title).toBe('제주')
    refetch()
    refetch()
    await tick(10_000)
    expect(saves()).toHaveLength(0)

    await act(async () => {
      await expect(result.current.assignTemplate('template-a')).rejects.toThrow()
    })
    expect(saves()).toHaveLength(0)
  })

  // T339's measured loop: the refused text must not come back with the refetch that reads the
  // post published, or it is queued again.
  it('never sends a text the lock refused a second time', async () => {
    const { result, saves, refetch } = setup(EXISTING, {
      posts: [EXISTING],
      publishOnDraftSave: EXISTING.slug,
    })

    act(() => result.current.setTitle('제주 3일'))
    await tick(AUTOSAVE_DEBOUNCE_MS)
    expect(saves()).toHaveLength(1)
    expect(result.current.state).toBe('idle')
    // Taken back to the screen, not only in the queue.
    expect(result.current.title).toBe('제주')

    refetch('published')
    refetch('published')
    await tick(AUTOSAVE_RETRY_BASE_MS * 16)
    expect(saves()).toHaveLength(1)
  })

  it('saves again from the server’s values once the post reopens', async () => {
    const { result, saves, refetch } = setup(EXISTING, {
      posts: [EXISTING],
      publishOnDraftSave: EXISTING.slug,
    })
    act(() => result.current.setTitle('제주 3일'))
    await tick(AUTOSAVE_DEBOUNCE_MS)
    refetch('published')

    refetch('finalized')
    await tick(10_000)
    expect(saves()).toHaveLength(1)
    expect(result.current.title).toBe('제주')

    act(() => result.current.setTitle('제주 4일'))
    await tick(AUTOSAVE_DEBOUNCE_MS)
    // Sent: the fake's row is still published, so it is refused again, but the editor saves.
    expect(saves()).toHaveLength(2)
  })
})
