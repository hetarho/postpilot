import { createRef } from 'react'
import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { clearCaret, peekCaret, useCaretHandoff } from './caret-handoff'
import { EDITOR_HANDOFF_TTL_MS } from '../config'

afterEach(() => {
  clearCaret()
  cleanup()
  vi.useRealTimers()
})

function fields() {
  const title = createRef<HTMLTextAreaElement>()
  const memo = createRef<HTMLTextAreaElement>()
  for (const [ref, id] of [
    [title, 'title'],
    [memo, 'memo'],
  ] as const) {
    const element = document.createElement('textarea')
    element.id = id
    element.value = '제주 이틀'
    document.body.append(element)
    ;(ref as { current: HTMLTextAreaElement | null }).current = element
  }
  return { title, memo }
}

it('carries the caret from the field that was focused when the slug was minted', () => {
  const refs = fields()
  const view = renderHook(({ slug }: { slug?: string }) => useCaretHandoff(slug, refs), {
    initialProps: { slug: undefined as string | undefined },
  })
  refs.memo.current!.focus()
  refs.memo.current!.setSelectionRange(2, 4)
  act(() => view.result.current.stash('20260920-jeju'))
  expect(peekCaret('20260920-jeju')).toMatchObject({
    slug: '20260920-jeju',
    field: 'memo',
    selectionStart: 2,
    selectionEnd: 4,
  })
  // The editor the navigation mounts claims it once, and only for its own post.
  expect(peekCaret('another-post')).toBeUndefined()
  const next = fields()
  renderHook(() => useCaretHandoff('20260920-jeju', next))
  expect(document.activeElement).toBe(next.memo.current)
  expect(next.memo.current!.selectionStart).toBe(2)
  expect(peekCaret('20260920-jeju')).toBeUndefined()
})

it('stashes nothing when neither field was focused, and drops a handoff nobody claimed', () => {
  const refs = fields()
  const view = renderHook(() => useCaretHandoff(undefined, refs))
  document.body.focus()
  act(() => view.result.current.stash('20260920-jeju'))
  expect(peekCaret('20260920-jeju')).toBeUndefined()

  refs.title.current!.focus()
  act(() => view.result.current.stash('20260920-jeju'))
  expect(peekCaret('20260920-jeju')?.field).toBe('title')
  vi.useFakeTimers()
  vi.setSystemTime(Date.now() + EDITOR_HANDOFF_TTL_MS + 1)
  expect(peekCaret('20260920-jeju')).toBeUndefined()
})
