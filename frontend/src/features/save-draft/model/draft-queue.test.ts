import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AUTOSAVE_DEBOUNCE_MS, AUTOSAVE_RETRY_BASE_MS } from '@/shared/config'
import type { ContentLanguage } from '@/shared/api'
import {
  type Draft,
  type SaveState,
  type SendDraft,
  attachDraftQueue,
  peekPendingDraft,
  discardDraftQueue,
  discardDraftQueues,
} from './draft-queue'

const EMPTY: Draft = { title: '', memo: '', answers: [] }

function draft(title: string, memo = '', answers: Draft['answers'] = []): Draft {
  return { title, memo, answers }
}

/** A backend whose failures and timing a test can decide. */
function backend(options: { failures?: number; mint?: string; holds?: number } = {}) {
  let failures = options.failures ?? 0
  let holds = options.holds ?? 0
  const sent: Array<{
    slug: string
    draft: Draft
    voiceId: string | undefined
    templateId: string | undefined
  }> = []
  const held: Array<() => void> = []
  const targets: Array<ContentLanguage | undefined> = []

  const send: SendDraft = async ({ slug, draft: value, voiceId, templateId, targetLanguage }) => {
    sent.push({ slug, draft: { ...value }, voiceId, templateId })
    targets.push(targetLanguage)
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
    templates: () => sent.map((call) => call.templateId),
    targets: () => targets,
    /** Lets the oldest held request finish. */
    open: () => held.shift()?.(),
  }
}

function attach(
  send: SendDraft,
  options: {
    slug?: string
    saved?: Draft
    voiceId?: string
    templateId?: string
    targetLanguage?: ContentLanguage
    retry?: (cause: unknown) => boolean
    onTakenBack?: () => void
  } = {},
) {
  const states: SaveState[] = []
  const minted: string[] = []
  const handle = attachDraftQueue({
    slug: options.slug,
    saved: options.saved ?? EMPTY,
    voiceId: options.voiceId ?? 'voice-a',
    templateId: options.templateId ?? '',
    targetLanguage: options.targetLanguage ?? 'ko',
    send,
    retry: options.retry,
    onState: (state) => states.push(state),
    onMinted: (slug) => minted.push(slug),
    onTakenBack: options.onTakenBack,
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

describe('target language', () => {
  it('follows an in-flight text save with the newest target and never resends the old target', async () => {
    const api = backend({ holds: 1 })
    const { handle } = attach(api.send, {
      slug: 'p',
      saved: draft('제주'),
      targetLanguage: 'ko',
    })

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.targets()).toEqual([undefined])

    const changed = handle.assign('targetLanguage', 'en')
    api.open()
    await advance(0)
    await expect(changed).resolves.toBeUndefined()

    expect(api.targets()).toEqual([undefined, 'en'])
    expect(api.titles()).toEqual(['제주 3일', '제주 3일'])
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
  // spec/legacy/policy/posts.md: a create always names its voice; an ordinary edit leaves it alone.
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

    await expect(handle.assign('voiceId', 'voice-b')).resolves.toBeUndefined()
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

    await handle.assign('voiceId', 'voice-b')
    api.open()
    await advance(0)

    expect(api.sent[1]).toEqual({ slug: '20260828-제주', draft: draft('제주'), voiceId: 'voice-b' })
    expect(handle.state()).toBe('saved')
  })

  it('reassigns an existing post at once and resolves when it lands', async () => {
    const api = backend()
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), voiceId: 'voice-a' })

    const done = handle.assign('voiceId', 'voice-b')
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

    const done = handle.assign('voiceId', 'voice-b')
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

    await expect(handle.assign('voiceId', 'voice-b')).rejects.toThrow('offline')
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

    const done = handle.assign('voiceId', 'voice-b')
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

    const done = handle.assign('voiceId', 'voice-b')
    discardDraftQueues()

    await expect(done).rejects.toThrow('session ended')
  })
})

// Plan 11 A12: the 템플릿 rides the same queue as the text, with one more state than the voice —
// a post may have none, so '' is a real value meaning "clear".
describe('the per-slug discard', () => {
  // The exception to "a queue outlives its editor, never its session" (tech/draft-autosave.md):
  // an intentional delete ends one slug's queue and nobody else's.
  it('stops the deleted slug retrying and leaves every other slug alone', async () => {
    // Never succeeds, so a queue that is still alive still holds its pending text.
    const api = backend({ failures: 100 })
    const deleted = attach(api.send, { slug: 'gone', saved: draft('제주') })
    const kept = attach(api.send, { slug: 'stays', saved: draft('부산') })

    deleted.handle.queue(draft('제주 3일'))
    kept.handle.queue(draft('부산 2일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent).toHaveLength(2)

    discardDraftQueue('gone')
    await advance(AUTOSAVE_RETRY_BASE_MS * 8)

    expect(api.sent.filter((call) => call.slug === 'gone')).toHaveLength(1)
    expect(api.sent.filter((call) => call.slug === 'stays').length).toBeGreaterThan(1)
    expect(peekPendingDraft('gone')).toBeUndefined()
    expect(peekPendingDraft('stays')).toEqual(draft('부산 2일'))
  })

  it("rejects the discarded queue's waiters with the delete reason", async () => {
    const api = backend({ holds: 1, failures: 1 })
    const { handle } = attach(api.send, { slug: 'gone', saved: draft('제주') })

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    const flushed = handle.flush()
    discardDraftQueue('gone')

    await expect(flushed).rejects.toThrow('post deleted')
  })

  it('is a no-op for a slug with no queue', () => {
    expect(() => discardDraftQueue('never-attached')).not.toThrow()
  })
})

/** A backend that refuses every save with `errors` in turn, then with its last one — the way the
 *  server answers every save of a published post (POST-86). `holds` keeps the first requests
 *  in flight until `open`. */
function refusing(options: { errors?: Error[]; holds?: number } = {}) {
  const locked = new Error('POST_PUBLISHED_LOCKED')
  const errors = options.errors ?? [locked]
  let holds = options.holds ?? 0
  const sent: Array<{
    draft: Draft
    voiceId: string | undefined
    templateId: string | undefined
    targetLanguage: ContentLanguage | undefined
  }> = []
  const held: Array<() => void> = []
  const send: SendDraft = async ({ draft: value, voiceId, templateId, targetLanguage }) => {
    sent.push({ draft: { ...value }, voiceId, templateId, targetLanguage })
    if (holds > 0) {
      holds -= 1
      await new Promise<void>((resolve) => held.push(resolve))
    }
    throw errors[Math.min(sent.length - 1, errors.length - 1)]
  }
  return {
    send,
    sent,
    locked,
    /** The editor's predicate: everything but the lock is worth another attempt. */
    retry: (cause: unknown) => cause !== locked,
    open: () => held.shift()?.(),
  }
}

describe('a refusal that is an answer', () => {
  // POST-86: the editor drops the refused text too, so it never shows or queues it again.
  it('tells the attached editor its text was taken back', async () => {
    const api = refusing()
    const onTakenBack = vi.fn()
    const { handle } = attach(api.send, {
      slug: 'p',
      saved: draft('제주'),
      retry: api.retry,
      onTakenBack,
    })
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(onTakenBack).toHaveBeenCalledTimes(1)

    // An outage is retried, never taken back.
    const offline = refusing({ errors: [new Error('offline')] })
    const told = vi.fn()
    const other = attach(offline.send, {
      slug: 'q',
      saved: draft('부산'),
      retry: offline.retry,
      onTakenBack: told,
    })
    other.handle.queue(draft('부산 2일'))
    await advance(AUTOSAVE_DEBOUNCE_MS + AUTOSAVE_RETRY_BASE_MS)
    expect(offline.sent.length).toBeGreaterThan(1)
    expect(told).not.toHaveBeenCalled()

    // A released editor is told nothing: a re-attached queue with no callback refuses quietly.
    handle.release()
    const again = attach(api.send, { slug: 'p', saved: draft('제주'), retry: api.retry })
    again.handle.queue(draft('제주 4일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent).toHaveLength(2)
    expect(onTakenBack).toHaveBeenCalledTimes(1)
  })

  it('drops the text, reports no failure and schedules no retry', async () => {
    const api = refusing()
    const { handle, states } = attach(api.send, {
      slug: 'p',
      saved: draft('제주'),
      retry: api.retry,
    })

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent).toHaveLength(1)
    // Nothing was ever saved by this queue, so it is quiet — never 다시 시도 중.
    expect(handle.state()).toBe('idle')
    expect(states).not.toContain('error')
    expect(peekPendingDraft('p')).toBeUndefined()

    await advance(AUTOSAVE_RETRY_BASE_MS * 16)
    expect(api.sent).toHaveLength(1)
  })

  it('takes back every assignment still waiting and rejects every waiter', async () => {
    const api = refusing({ holds: 1 })
    const { handle } = attach(api.send, {
      slug: 'p',
      saved: draft('제주'),
      voiceId: 'voice-a',
      templateId: '',
      targetLanguage: 'ko',
      retry: api.retry,
    })

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    // Chosen while the text save is out, so none of them is on the request that is refused.
    const settled = Promise.all([
      expect(handle.assign('voiceId', 'voice-b')).rejects.toBe(api.locked),
      expect(handle.assign('templateId', 'template-1')).rejects.toBe(api.locked),
      expect(handle.assign('targetLanguage', 'en')).rejects.toBe(api.locked),
      expect(handle.flush()).rejects.toBe(api.locked),
    ])
    api.open()
    await advance(0)
    await settled

    await advance(AUTOSAVE_RETRY_BASE_MS * 16)
    expect(api.sent).toHaveLength(1)
    expect(handle.state()).toBe('idle')

    // What the next save carries is text alone: the queue holds the server's assignments again.
    handle.queue(draft('제주 4일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent[1]).toEqual({
      draft: draft('제주 4일'),
      voiceId: undefined,
      templateId: undefined,
      targetLanguage: undefined,
    })
  })

  it('still retries every other failure', async () => {
    const offline = new Error('offline')
    const api = refusing({ errors: [offline, offline] })
    const { handle } = attach(api.send, { slug: 'p', saved: draft('제주'), retry: api.retry })

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(handle.state()).toBe('error')
    await advance(AUTOSAVE_RETRY_BASE_MS)
    expect(api.sent).toHaveLength(2)
  })

  // A queue outlives its editor, and the editor that adopts it brings its own answer rule, as it
  // brings its own transport.
  it('asks the editor attached now, not the one that queued the text', async () => {
    const api = refusing()
    const first = attach(api.send, { slug: 'p', saved: draft('제주') })
    first.handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    // The first editor gave no rule, so the refusal read as an outage and a retry is waiting.
    expect(first.handle.state()).toBe('error')
    first.handle.release()

    const second = attach(api.send, { slug: 'p', saved: draft('제주'), retry: api.retry })
    await advance(AUTOSAVE_RETRY_BASE_MS)
    expect(api.sent).toHaveLength(2)
    expect(second.handle.state()).toBe('idle')
    await advance(AUTOSAVE_RETRY_BASE_MS * 16)
    expect(api.sent).toHaveLength(2)
  })
})
