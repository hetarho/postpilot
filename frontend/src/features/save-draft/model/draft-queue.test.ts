import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AUTOSAVE_DEBOUNCE_MS, AUTOSAVE_RETRY_BASE_MS } from '@/shared/config'
import {
  type Draft,
  type SaveState,
  type SendDraft,
  attachDraftQueue,
  peekPendingDraft,
  discardDraftQueues,
} from './draft-queue'

const EMPTY: Draft = { title: '', memo: '' }

function draft(title: string, memo = ''): Draft {
  return { title, memo }
}

/** A backend whose failures and timing a test can decide. */
function backend(options: { failures?: number; mint?: string; holds?: number } = {}) {
  let failures = options.failures ?? 0
  let holds = options.holds ?? 0
  const sent: Array<{ slug: string; draft: Draft }> = []
  const held: Array<() => void> = []

  const send: SendDraft = async (slug, value) => {
    sent.push({ slug, draft: { ...value } })
    if (holds > 0) {
      holds -= 1
      await new Promise<void>((resolve) => held.push(resolve))
    }
    if (failures > 0) {
      failures -= 1
      throw new Error('offline')
    }
    return slug || (options.mint ?? '20260828-minted')
  }

  return {
    send,
    sent,
    titles: () => sent.map((call) => call.draft.title),
    /** Lets the oldest held request finish. */
    open: () => held.shift()?.(),
  }
}

function attach(
  send: SendDraft,
  options: { slug?: string; saved?: Draft } = {},
) {
  const states: SaveState[] = []
  const minted: string[] = []
  const handle = attachDraftQueue({
    slug: options.slug,
    saved: options.saved ?? EMPTY,
    send,
    onState: (state) => states.push(state),
    onMinted: (slug) => minted.push(slug),
  })
  return { handle, states, minted }
}

async function advance(ms: number) {
  await vi.advanceTimersByTimeAsync(ms)
}

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  discardDraftQueues()
  vi.useRealTimers()
})

describe('the debounce', () => {
  it('sends once for a burst of typing, carrying the newest text', async () => {
    const api = backend()
    const { handle } = attach(api.send, { slug: 'p' })

    handle.queue(draft('제'))
    await advance(AUTOSAVE_DEBOUNCE_MS - 1)
    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS - 1)
    expect(api.sent).toHaveLength(0)

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.titles()).toEqual(['제주 3일'])
    expect(handle.state()).toBe('saved')
  })

  it('sends nothing while the text matches what the server holds', async () => {
    const api = backend()
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주', '첫날') })

    handle.queue(draft('제주', '첫날'))
    await advance(10_000)

    expect(api.sent).toHaveLength(0)
    expect(handle.state()).toBe('idle')
  })

  // Otherwise the editor would sit on "저장 대기 중" with nothing left to send.
  it('goes quiet again when the text is typed back to the saved value', async () => {
    const api = backend()
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주') })

    handle.queue(draft('제주도'))
    expect(handle.state()).toBe('dirty')
    handle.queue(draft('제주'))

    expect(handle.state()).toBe('idle')
    await advance(10_000)
    expect(api.sent).toHaveLength(0)
  })
})

describe('retries', () => {
  it('reports the failure and retries with a doubling delay', async () => {
    const api = backend({ failures: 99 })
    const { handle } = attach(api.send, { slug: 'p' })

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent).toHaveLength(1)
    expect(handle.state()).toBe('error')

    await advance(AUTOSAVE_RETRY_BASE_MS)
    expect(api.sent).toHaveLength(2)

    await advance(AUTOSAVE_RETRY_BASE_MS)
    expect(api.sent).toHaveLength(2) // the second delay is twice as long
    await advance(AUTOSAVE_RETRY_BASE_MS)
    expect(api.sent).toHaveLength(3)
  })

  // The documented promise of the backoff is that a dead network does not become a
  // request loop — which typing would defeat if each keystroke restarted the debounce.
  it('is not restarted by typing during an outage', async () => {
    const api = backend({ failures: 99 })
    const { handle } = attach(api.send, { slug: 'p' })

    handle.queue(draft('a'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent).toHaveLength(1)

    for (let n = 0; n < 20; n += 1) {
      handle.queue(draft(`a${n}`))
      await advance(100)
    }

    // 2 s of typing covers the first (1 s) and second (2 s) retry windows and nothing
    // more; without the guard every keystroke would have sent a request of its own.
    expect(api.sent).toHaveLength(2)
  })

  it('retries with the newest text, not the snapshot that failed', async () => {
    const api = backend({ failures: 1 })
    const { handle } = attach(api.send, { slug: 'p' })

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_RETRY_BASE_MS)

    expect(api.titles()).toEqual(['제주', '제주 3일'])
    expect(handle.state()).toBe('saved')
  })

  // The text belongs to the user, not to the component that happened to be mounted.
  it('keeps retrying after the editor is gone', async () => {
    const api = backend({ failures: 1 })
    const { handle } = attach(api.send, { slug: 'p' })

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent).toHaveLength(1)

    handle.release()
    await advance(AUTOSAVE_RETRY_BASE_MS)

    expect(api.titles()).toEqual(['제주', '제주'])
  })

  it('hands an unfinished save to the next editor for the same post', async () => {
    const first = backend({ failures: 99 })
    const { handle } = attach(first.send, { slug: 'p' })
    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    handle.release()

    expect(peekPendingDraft('p')).toEqual(draft('제주'))

    // The new editor was told the server's older text, but it must not start a second
    // chain of saves, and its own transport must take over.
    const second = backend()
    const reattached = attach(second.send, { slug: 'p', saved: EMPTY })
    await advance(AUTOSAVE_RETRY_BASE_MS * 8)

    expect(second.titles()).toEqual(['제주'])
    expect(reattached.handle.state()).toBe('saved')
  })
})

describe('teardown', () => {
  it('sends immediately instead of waiting for the debounce', async () => {
    const api = backend()
    const { handle } = attach(api.send, { slug: 'p' })

    handle.queue(draft('제주'))
    handle.saveNow()
    await advance(0)

    expect(api.titles()).toEqual(['제주'])
  })

  // Without this the characters typed during the last request would never go out: nobody
  // is left to wait for another debounce.
  it('sends the leftover text as soon as the request in flight lands', async () => {
    const api = backend({ holds: 1 })
    const { handle } = attach(api.send, { slug: 'p' })

    handle.queue(draft('제주'))
    handle.saveNow()
    await advance(0)
    expect(api.titles()).toEqual(['제주'])

    handle.queue(draft('제주 3일'))
    handle.saveNow()
    api.open()
    await advance(0)

    expect(api.titles()).toEqual(['제주', '제주 3일'])
  })
})

describe('an undo while a save is out', () => {
  // A request already sent cannot be recalled, so the undo has to be sent as its own
  // save. Comparing it with the older `saved` would call it "already saved" and the
  // server would keep the text the user just took back.
  it('is sent rather than treated as already saved', async () => {
    const api = backend({ holds: 1 })
    const { handle } = attach(api.send, { slug: 'p', saved: draft('A') })

    handle.queue(draft('AB'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.titles()).toEqual(['AB'])

    handle.queue(draft('A'))
    api.open()
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.titles()).toEqual(['AB', 'A'])
    expect(handle.state()).toBe('saved')
  })
})

describe('the end of a session', () => {
  // A retry firing after someone else has signed in on this device would send the
  // previous account's text under the new account's cookie.
  it('stops every retry', async () => {
    const api = backend({ failures: 99 })
    const { handle } = attach(api.send, { slug: 'p' })

    handle.queue(draft('비밀'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent).toHaveLength(1)

    discardDraftQueues()
    await advance(AUTOSAVE_RETRY_BASE_MS * 20)

    expect(api.sent).toHaveLength(1)
  })

  it('is not undone by a request that lands afterwards', async () => {
    const api = backend({ holds: 1, mint: '20260828-비밀' })
    const { handle, minted } = attach(api.send)

    handle.queue(draft('비밀'))
    handle.saveNow()
    await advance(0)

    discardDraftQueues()
    api.open()
    await advance(0)

    expect(minted).toEqual([])
    expect(peekPendingDraft('20260828-비밀')).toBeUndefined()
  })
})

describe('the first save of a new draft', () => {
  it('mints a slug once and writes to it afterwards', async () => {
    const api = backend({ mint: '20260828-제주' })
    const { handle, minted } = attach(api.send)

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(minted).toEqual(['20260828-제주'])
    expect(api.sent[0].slug).toBe('')

    handle.queue(draft('제주', '첫날'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.sent[1].slug).toBe('20260828-제주')
    expect(minted).toHaveLength(1)
  })

  // Two "새 글" editors are two different drafts; sharing a queue would let the second
  // one clear the first one's unfinished work, or claim the slug it minted.
  it('keeps two unsaved drafts apart', async () => {
    const first = backend({ failures: 99 })
    const a = attach(first.send)
    a.handle.queue(draft('첫 번째'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    a.handle.release()

    const second = backend({ mint: '20260828-두-번째' })
    const b = attach(second.send)
    b.handle.queue(draft('두 번째'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(second.titles()).toEqual(['두 번째'])
    expect(b.minted).toEqual(['20260828-두-번째'])

    await advance(AUTOSAVE_RETRY_BASE_MS * 4)
    expect(first.sent.length).toBeGreaterThan(1)
    expect(first.titles().every((title) => title === '첫 번째')).toBe(true)
  })

  // Two creates for one draft would leave a second post nobody can see.
  it('creates once even when a teardown lands mid-create', async () => {
    const api = backend({ holds: 1, mint: '20260828-제주' })
    const { handle, minted } = attach(api.send)

    handle.queue(draft('제주'))
    handle.saveNow()
    handle.saveNow()
    await advance(0)
    expect(api.sent).toHaveLength(1)

    api.open()
    await advance(0)

    expect(minted).toEqual(['20260828-제주'])
    expect(api.sent).toHaveLength(1)
  })
})
