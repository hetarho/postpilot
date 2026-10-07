import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import type { ReturnContext } from '@/shared/lib/navigation'
import { useHistoryScrollReturn } from './useHistoryScrollReturn'

const context = vi.hoisted(() => ({ entry: undefined as ReturnContext | undefined }))
const complete = vi.hoisted(() =>
  vi.fn(() => {
    context.entry = undefined
  }),
)
vi.mock('@/entities/post', () => ({
  readPostHistoryReturn: () => context.entry,
  completePostHistoryReturn: complete,
}))

const frames = new Map<number, FrameRequestCallback>()
let height = 900
const originalHeight = Object.getOwnPropertyDescriptor(document.documentElement, 'scrollHeight')
const initial = {
  posts: [] as { slug: string }[],
  isPending: false,
  isFetching: false,
  isError: false,
  hasNextPage: false,
  isFetchingNextPage: false,
  isFetchNextPageError: false,
  fetchNextPage: vi.fn(),
}
function remember(filters: Record<string, string> = {}) {
  context.entry = {
    version: 1,
    ownerKey: 'alice',
    path: '/posts',
    section: 'posts',
    targetId: 'target',
    scrollY: 1400,
    filters: { ...filters, restoreScroll: 'true' },
  }
}
function paint() {
  act(() => {
    for (const [id, callback] of [...frames]) {
      frames.delete(id)
      callback(0)
    }
  })
}

beforeEach(() => {
  context.entry = undefined
  complete.mockClear()
  initial.fetchNextPage.mockClear()
  frames.clear()
  height = 900
  let next = 0
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
    const id = ++next
    frames.set(id, callback)
    return id
  })
  vi.stubGlobal('cancelAnimationFrame', (id: number) => frames.delete(id))
  vi.stubGlobal('innerHeight', 900)
  Object.defineProperty(document.documentElement, 'scrollHeight', {
    configurable: true,
    get: () => height,
  })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
})
afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  if (originalHeight)
    Object.defineProperty(document.documentElement, 'scrollHeight', originalHeight)
  else Reflect.deleteProperty(document.documentElement, 'scrollHeight')
})

describe('cold history scroll return', () => {
  it('waits for the delayed matching first page and target pagination before restoring once', () => {
    remember({ q: '제주', status: 'review' })
    const first = Array.from({ length: 20 }, (_, index) => ({ slug: `post-${index}` }))
    const view = renderHook(
      ({ list }) => useHistoryScrollReturn('alice', { q: '제주', status: 'review' }, '제주', list),
      {
        initialProps: {
          list: { ...initial, isPending: true, isFetching: true, hasNextPage: true },
        },
      },
    )
    paint()
    expect(window.scrollTo).not.toHaveBeenCalled()
    expect(initial.fetchNextPage).not.toHaveBeenCalled()
    view.rerender({ list: { ...initial, posts: first, hasNextPage: true } })
    paint()
    expect(initial.fetchNextPage).toHaveBeenCalledOnce()
    expect(complete).not.toHaveBeenCalled()
    view.rerender({
      list: {
        ...initial,
        posts: first,
        hasNextPage: true,
        isFetching: true,
        isFetchingNextPage: true,
      },
    })
    paint()
    expect(window.scrollTo).not.toHaveBeenCalled()
    height = 2600
    view.rerender({
      list: { ...initial, posts: [...first, { slug: 'target' }], hasNextPage: true },
    })
    paint()
    expect(window.scrollTo).toHaveBeenCalledExactlyOnceWith(0, 1400)
    expect(complete).toHaveBeenCalledOnce()
    view.rerender({
      list: { ...initial, posts: [...first, { slug: 'target' }], hasNextPage: true },
    })
    paint()
    expect(window.scrollTo).toHaveBeenCalledOnce()
  })

  it('preserves a pending return while the URL or settled request belongs to another narrowing', () => {
    remember({ q: '제주', status: 'review' })
    const view = renderHook(
      ({ q, settled }) =>
        useHistoryScrollReturn('alice', { q, status: 'review' }, settled, {
          ...initial,
          posts: [{ slug: 'target' }],
          hasNextPage: true,
        }),
      { initialProps: { q: '부산', settled: '부산' } },
    )
    paint()
    expect(window.scrollTo).not.toHaveBeenCalled()
    view.rerender({ q: '제주', settled: '부산' })
    paint()
    expect(window.scrollTo).not.toHaveBeenCalled()
    expect(complete).not.toHaveBeenCalled()
    expect(context.entry).toBeDefined()
    height = 2600
    view.rerender({ q: '제주', settled: '제주' })
    paint()
    expect(window.scrollTo).toHaveBeenCalledExactlyOnceWith(0, 1400)
  })

  it('loads enough older pages for the retained position when an edited target is already newest', () => {
    remember()
    const first = [
      { slug: 'target' },
      ...Array.from({ length: 19 }, (_, index) => ({ slug: `older-${index}` })),
    ]
    const view = renderHook(({ list }) => useHistoryScrollReturn('alice', {}, '', list), {
      initialProps: { list: { ...initial, posts: first, hasNextPage: true } },
    })
    paint()
    expect(initial.fetchNextPage).toHaveBeenCalledOnce()
    expect(window.scrollTo).not.toHaveBeenCalled()
    expect(complete).not.toHaveBeenCalled()
    view.rerender({
      list: {
        ...initial,
        posts: first,
        hasNextPage: true,
        isFetching: true,
        isFetchingNextPage: true,
      },
    })
    paint()
    expect(initial.fetchNextPage).toHaveBeenCalledOnce()
    expect(context.entry).toBeDefined()
    // Even the next page can be shorter than the document position being restored.
    height = 1800
    const second = [
      ...first,
      ...Array.from({ length: 20 }, (_, index) => ({ slug: `second-${index}` })),
    ]
    view.rerender({ list: { ...initial, posts: second, hasNextPage: true } })
    paint()
    expect(initial.fetchNextPage).toHaveBeenCalledTimes(2)
    expect(window.scrollTo).not.toHaveBeenCalled()
    expect(complete).not.toHaveBeenCalled()
    height = 2400
    view.rerender({
      list: { ...initial, posts: [...second, { slug: 'third-page' }], hasNextPage: true },
    })
    paint()
    expect(window.scrollTo).toHaveBeenCalledExactlyOnceWith(0, 1400)
    expect(complete).toHaveBeenCalledOnce()
    expect(initial.fetchNextPage).toHaveBeenCalledTimes(2)
  })

  it('keeps a failed page pending and lets an explicit retry finish without an automatic retry loop', () => {
    remember()
    const first = [{ slug: 'first' }]
    const view = renderHook(({ list }) => useHistoryScrollReturn('alice', {}, '', list), {
      initialProps: { list: { ...initial, posts: first, hasNextPage: true } },
    })
    paint()
    expect(initial.fetchNextPage).toHaveBeenCalledOnce()
    view.rerender({
      list: { ...initial, posts: first, hasNextPage: true, isFetchNextPageError: true },
    })
    paint()
    view.rerender({
      list: { ...initial, posts: first, hasNextPage: true, isFetchNextPageError: true },
    })
    paint()
    expect(initial.fetchNextPage).toHaveBeenCalledOnce()
    expect(window.scrollTo).not.toHaveBeenCalled()
    expect(context.entry).toBeDefined()
    view.rerender({
      list: {
        ...initial,
        posts: first,
        hasNextPage: true,
        isFetching: true,
        isFetchingNextPage: true,
      },
    })
    paint()
    view.rerender({
      list: { ...initial, posts: [...first, { slug: 'target' }], hasNextPage: false },
    })
    paint()
    expect(window.scrollTo).toHaveBeenCalledExactlyOnceWith(0, 1400)
    expect(complete).toHaveBeenCalledOnce()
  })

  it('restores without loading extra pages when the document already accommodates the saved position', () => {
    remember()
    height = 2600
    renderHook(() =>
      useHistoryScrollReturn('alice', {}, '', {
        ...initial,
        posts: [{ slug: 'first' }],
        hasNextPage: true,
      }),
    )
    paint()
    expect(initial.fetchNextPage).not.toHaveBeenCalled()
    expect(window.scrollTo).toHaveBeenCalledExactlyOnceWith(0, 1400)
  })

  it('finishes an exhausted deleted-target return and leaves ordinary arrivals untouched', () => {
    const view = renderHook(({ list }) => useHistoryScrollReturn('alice', {}, '', list), {
      initialProps: { list: { ...initial } },
    })
    paint()
    expect(window.scrollTo).not.toHaveBeenCalled()
    remember()
    view.rerender({ list: { ...initial, posts: [{ slug: 'remaining' }] } })
    paint()
    expect(window.scrollTo).toHaveBeenCalledExactlyOnceWith(0, 1400)
    expect(complete).toHaveBeenCalledOnce()
  })
})
