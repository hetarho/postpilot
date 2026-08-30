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
  const sent: Array<{
    slug: string
    draft: Draft
    voiceId: string | undefined
    purposeId: string | undefined
  }> = []
  const held: Array<() => void> = []

  const send: SendDraft = async (slug, value, voiceId, purposeId) => {
    sent.push({ slug, draft: { ...value }, voiceId, purposeId })
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
    voices: () => sent.map((call) => call.voiceId),
    purposes: () => sent.map((call) => call.purposeId),
    /** Lets the oldest held request finish. */
    open: () => held.shift()?.(),
  }
}

function attach(
  send: SendDraft,
  options: { slug?: string; saved?: Draft; voiceId?: string; purposeId?: string } = {},
) {
  const states: SaveState[] = []
  const minted: string[] = []
  const handle = attachDraftQueue({
    slug: options.slug,
    saved: options.saved ?? EMPTY,
    voiceId: options.voiceId ?? 'voice-a',
    purposeId: options.purposeId ?? '',
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

describe('an explicit flush', () => {
  it('sends immediately and resolves only after the latest draft lands', async () => {
    const api = backend({ holds: 1 })
    const { handle } = attach(api.send, { slug: 'p' })
    handle.queue(draft('생성에 쓸 최신 메모'))

    let settled = false
    const flushed = handle.flush().then(() => {
      settled = true
    })
    await advance(0)

    expect(api.titles()).toEqual(['생성에 쓸 최신 메모'])
    expect(settled).toBe(false)

    api.open()
    await flushed
    expect(settled).toBe(true)
    expect(handle.state()).toBe('saved')
  })

  it('rejects the action on a failed attempt while autosave keeps retrying', async () => {
    const api = backend({ failures: 1 })
    const { handle } = attach(api.send, { slug: 'p' })
    handle.queue(draft('제주'))

    await expect(handle.flush()).rejects.toThrow('offline')
    expect(handle.state()).toBe('error')

    await advance(AUTOSAVE_RETRY_BASE_MS)
    expect(api.titles()).toEqual(['제주', '제주'])
    expect(handle.state()).toBe('saved')
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

describe('the voice assignment', () => {
  // spec/policy/posts.md: a create always names its voice; an ordinary edit leaves it alone.
  it('sends the voice with the create and not with an unchanged edit', async () => {
    const api = backend({ mint: '20260828-제주' })
    const { handle } = attach(api.send, { voiceId: 'voice-a' })

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.voices()).toEqual(['voice-a', undefined])
  })

  it('lets a draft with no post yet change its mind without sending anything', async () => {
    const api = backend()
    const { handle } = attach(api.send, { voiceId: 'voice-a' })

    await expect(handle.assignVoice('voice-b')).resolves.toBeUndefined()
    await advance(10_000)
    expect(api.sent).toHaveLength(0)

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.voices()).toEqual(['voice-b'])
  })

  // The choice landed during the create round trip, so the create carried the old one.
  it('follows a voice chosen while the create was out with an immediate reassignment', async () => {
    const api = backend({ holds: 1, mint: '20260828-제주' })
    const { handle } = attach(api.send, { voiceId: 'voice-a' })
    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.voices()).toEqual(['voice-a'])

    await handle.assignVoice('voice-b')
    api.open()
    await advance(0)

    expect(api.sent[1]).toEqual({ slug: '20260828-제주', draft: draft('제주'), voiceId: 'voice-b' })
    expect(handle.state()).toBe('saved')
  })

  it('reassigns an existing post at once and resolves when it lands', async () => {
    const api = backend()
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), voiceId: 'voice-a' })

    const done = handle.assignVoice('voice-b')
    await advance(0)

    expect(api.sent).toEqual([{ slug: 'p', draft: draft('제주'), voiceId: 'voice-b' }])
    await expect(done).resolves.toBeUndefined()
    expect(handle.state()).toBe('saved')

    // The assignment is settled: later text goes out without it.
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.voices()).toEqual(['voice-b', undefined])
  })

  // The bug the queue exists to prevent: a title save that left before the reassignment
  // must not carry the old voice back over it when its turn comes.
  it('does not let text typed during a reassignment revert it', async () => {
    const api = backend({ holds: 1 })
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), voiceId: 'voice-a' })

    const done = handle.assignVoice('voice-b')
    await advance(0)
    handle.queue(draft('제주 3일'))
    api.open()
    await done
    // The text typed meanwhile is an ordinary keystroke: it follows after the debounce.
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.sent).toEqual([
      { slug: 'p', draft: draft('제주'), voiceId: 'voice-b' },
      { slug: 'p', draft: draft('제주 3일'), voiceId: undefined },
    ])
  })

  it('takes a refused reassignment back so the next save carries text only', async () => {
    const api = backend({ failures: 1 })
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), voiceId: 'voice-a' })

    await expect(handle.assignVoice('voice-b')).rejects.toThrow('offline')
    // Nothing else changed, so there is nothing left to retry either — and nothing was ever
    // saved by this queue, so it is quiet rather than "저장됨".
    await advance(AUTOSAVE_RETRY_BASE_MS * 4)
    expect(api.sent).toHaveLength(1)
    expect(handle.state()).toBe('idle')

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent[1]).toEqual({ slug: 'p', draft: draft('제주 3일'), voiceId: undefined })
  })

  it('does not stand down on unchanged text while a reassignment is pending', async () => {
    const api = backend()
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), voiceId: 'voice-a' })

    const done = handle.assignVoice('voice-b')
    // Typed and typed back before the request could leave: still a reassignment to send.
    handle.queue(draft('제주도'))
    handle.queue(draft('제주'))
    await advance(0)

    await expect(done).resolves.toBeUndefined()
    expect(api.voices()).toEqual(['voice-b'])
  })

  it('rejects a reassignment when the session ends first', async () => {
    const api = backend({ holds: 1 })
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), voiceId: 'voice-a' })

    const done = handle.assignVoice('voice-b')
    discardDraftQueues()

    await expect(done).rejects.toThrow('session ended')
  })
})

// Plan 11 A12: the 용도 rides the same queue as the text, with one more state than the voice —
// a post may have none, so '' is a real value meaning "clear".
describe('the post purpose', () => {
  it('sends nothing on a create that stayed on 없음', async () => {
    const api = backend()
    const { handle } = attach(api.send, { purposeId: '' })

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    // Omitted, not '': a create has no assignment to clear, so the request is exactly what it
    // was before purposes existed.
    expect(api.purposes()).toEqual([undefined])
  })

  it('carries a purpose chosen before the post exists into the create', async () => {
    const api = backend()
    const { handle } = attach(api.send, { purposeId: '' })

    await expect(handle.assignPurpose('purpose-a')).resolves.toBeUndefined()
    await advance(10_000)
    expect(api.sent).toHaveLength(0)

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.purposes()).toEqual(['purpose-a'])
  })

  it('assigns an existing post at once, then leaves later saves alone', async () => {
    const api = backend()
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), purposeId: '' })

    const done = handle.assignPurpose('purpose-a')
    await advance(0)
    await expect(done).resolves.toBeUndefined()
    expect(api.purposes()).toEqual(['purpose-a'])

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.purposes()).toEqual(['purpose-a', undefined])
  })

  it('sends an empty string to clear an assignment', async () => {
    const api = backend()
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), purposeId: 'purpose-a' })

    await handle.assignPurpose('')
    await advance(0)

    // Present-and-empty, which is what the server reads as 없음 — distinct from omitting it.
    expect(api.purposes()).toEqual([''])
  })

  // The bug this shares with the voice: a title save that left before the selection must not
  // carry the old assignment back over it.
  it('does not let text typed during an assignment revert it', async () => {
    const api = backend({ holds: 1 })
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), purposeId: '' })

    const done = handle.assignPurpose('purpose-a')
    await advance(0)
    handle.queue(draft('제주 3일'))
    api.open()
    await done
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.sent).toEqual([
      { slug: 'p', draft: draft('제주'), voiceId: undefined, purposeId: 'purpose-a' },
      { slug: 'p', draft: draft('제주 3일'), voiceId: undefined, purposeId: undefined },
    ])
  })

  it('takes a refused assignment back so the next save carries text only', async () => {
    const api = backend({ failures: 1 })
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), purposeId: '' })

    await expect(handle.assignPurpose('purpose-a')).rejects.toThrow('offline')
    await advance(AUTOSAVE_RETRY_BASE_MS * 4)
    expect(api.sent).toHaveLength(1)

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent[1]).toEqual({
      slug: 'p',
      draft: draft('제주 3일'),
      voiceId: undefined,
      purposeId: undefined,
    })
  })
})
