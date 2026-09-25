import { afterEach, beforeEach, describe, expect, expectTypeOf, it, vi } from 'vitest'
import type { ContentLanguage } from '@/shared/api'
import { AUTOSAVE_DEBOUNCE_MS, AUTOSAVE_RETRY_BASE_MS } from '@/shared/config'
import {
  type Assignments,
  type Draft,
  type DraftQueueOptions,
  type SendDraft,
  attachDraftQueue,
  discardDraftQueue,
  discardDraftQueues,
} from './draft-queue'

const SLUG = '20260301-jeju'

/** Every channel's baseline but the one under test, which each row sets over it. */
const OTHER_BASELINES: Assignments = { voiceId: 'voice-a', templateId: '', targetLanguage: 'ko' }

function draft(title: string): Draft {
  return { title, memo: '', answers: [] }
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

// A channel whose create rule is "sent only when chosen, and '' clears": the 템플릿 today. Every
// assignment is one record walked by one channel list, so a future channel with this rule is one
// more row.
describe.each([
  {
    name: '템플릿',
    channel: 'templateId',
    values: ['template-a', 'template-b', 'template-c'],
    noun: 'template',
  },
] as const)('the post $name', ({ channel, values, noun }) => {
  const [first, second, third] = values

  /** A backend that records the title and this channel's value each save carried. Its failures
   *  and timing are the test's to decide. */
  function backend(
    options: { failures?: number; holds?: number; mint?: string; refusal?: Error } = {},
  ) {
    let failures = options.failures ?? 0
    let holds = options.holds ?? 0
    const sent: Array<{ slug: string; title: string; value: string | undefined }> = []
    const held: Array<() => void> = []
    const send: SendDraft = async (request) => {
      sent.push({ slug: request.slug, title: request.draft.title, value: request[channel] })
      if (holds > 0) {
        holds -= 1
        await new Promise<void>((resolve) => held.push(resolve))
      }
      if (failures > 0) {
        failures -= 1
        throw options.refusal ?? new Error('offline')
      }
      return request.slug || (options.mint ?? '20260828-minted')
    }
    return {
      send,
      sent,
      values: () => sent.map((call) => call.value),
      /** Lets the oldest held request finish. */
      open: () => held.shift()?.(),
    }
  }

  function attach(
    send: SendDraft,
    options: {
      slug?: string
      saved?: Draft
      baseline?: string
      retry?: (cause: unknown) => boolean
    } = {},
  ) {
    return attachDraftQueue({
      ...OTHER_BASELINES,
      [channel]: options.baseline ?? '',
      slug: options.slug,
      saved: options.saved ?? draft(''),
      send,
      retry: options.retry,
      onState: () => {},
      onMinted: () => {},
    })
  }

  it('sends nothing on a create that stayed on 없음', async () => {
    const api = backend()
    const handle = attach(api.send)

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    // Omitted, not '': a create has nothing to clear.
    expect(api.values()).toEqual([undefined])
  })

  it('carries a choice made before the post exists into the create', async () => {
    const api = backend()
    const handle = attach(api.send)

    await expect(handle.assign(channel, first)).resolves.toBeUndefined()
    await advance(10_000)
    expect(api.sent).toHaveLength(0)

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.values()).toEqual([first])

    // The create settled it: the next save is text alone.
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.values()).toEqual([first, undefined])
  })

  // A choice is not text: picked, typed and erased, a new draft still has nothing to create.
  it('creates no post for a choice alone', async () => {
    const api = backend()
    const handle = attach(api.send)

    await handle.assign(channel, first)
    handle.queue(draft('제'))
    handle.queue(draft(''))
    await advance(10_000)

    expect(api.sent).toHaveLength(0)
  })

  // A create is retried whole, since there is no post yet to take a choice back to.
  it('keeps a choice on a refused create and retries with it', async () => {
    const api = backend({ failures: 1 })
    const handle = attach(api.send)

    await handle.assign(channel, first)
    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    await advance(AUTOSAVE_RETRY_BASE_MS)

    expect(api.values()).toEqual([first, first])
  })

  // The choice landed during the create round trip, so the create carried the old one.
  it('follows a choice made while the create was out with an immediate update', async () => {
    const api = backend({ holds: 1, mint: '20260828-제주' })
    const handle = attach(api.send)
    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.values()).toEqual([undefined])

    await handle.assign(channel, first)
    api.open()
    // No debounce: an assignment is an action, not a keystroke.
    await advance(0)

    expect(api.sent[1]).toEqual({ slug: '20260828-제주', title: '제주', value: first })
    expect(handle.state()).toBe('saved')
  })

  it('assigns an existing post at once, then leaves later saves alone', async () => {
    const api = backend()
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    const done = handle.assign(channel, first)
    await advance(0)
    await expect(done).resolves.toBeUndefined()
    expect(api.sent).toEqual([{ slug: SLUG, title: '제주', value: first }])

    // The value the server now holds is no change at all.
    await handle.assign(channel, first)
    await advance(0)
    expect(api.sent).toHaveLength(1)

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.values()).toEqual([first, undefined])
  })

  it('leaves an existing post’s value off an ordinary text save', async () => {
    const api = backend()
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주'), baseline: first })

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.sent).toEqual([{ slug: SLUG, title: '제주 3일', value: undefined }])
  })

  it('treats 없음 on a post that holds one as a clear', async () => {
    const api = backend()
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주'), baseline: first })

    // What the post already holds, as the editor was told: nothing to send.
    await handle.assign(channel, first)
    await advance(0)
    expect(api.sent).toHaveLength(0)

    await handle.assign(channel, '')
    await advance(0)

    // Present-and-empty, which the server reads as 없음 — distinct from omitting it.
    expect(api.values()).toEqual([''])
  })

  // The bug the queue exists to prevent: a title save that left before the choice must not carry
  // the old value back over it.
  it('does not let text typed during an assignment revert it', async () => {
    const api = backend({ holds: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    const done = handle.assign(channel, first)
    await advance(0)
    handle.queue(draft('제주 3일'))
    api.open()
    await done
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.sent).toEqual([
      { slug: SLUG, title: '제주', value: first },
      { slug: SLUG, title: '제주 3일', value: undefined },
    ])
  })

  it('takes a refused assignment back so the next save carries text only', async () => {
    const api = backend({ failures: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    await expect(handle.assign(channel, first)).rejects.toThrow('offline')
    // Nothing else changed, so nothing is retried.
    await advance(AUTOSAVE_RETRY_BASE_MS * 4)
    expect(api.sent).toHaveLength(1)

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent[1]).toEqual({ slug: SLUG, title: '제주 3일', value: undefined })
  })

  // The text typed back has nothing left to say, but a choice made behind the failing save still
  // does, so the retry is not stood down.
  it('retries a choice made behind a failing text save typed back meanwhile', async () => {
    const api = backend({ holds: 1, failures: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    const done = handle.assign(channel, first)
    handle.queue(draft('제주'))
    api.open()
    await advance(AUTOSAVE_RETRY_BASE_MS)

    expect(api.sent).toEqual([
      { slug: SLUG, title: '제주 3일', value: undefined },
      { slug: SLUG, title: '제주', value: first },
    ])
    await expect(done).resolves.toBeUndefined()
  })

  it('keeps a choice waiting through a backoff when the text is typed back', async () => {
    const api = backend({ holds: 1, failures: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    const done = handle.assign(channel, first)
    api.open()
    // The text save failed and a retry is waiting; the text goes back to what the server holds.
    await advance(0)
    handle.queue(draft('제주'))
    await advance(AUTOSAVE_RETRY_BASE_MS)

    expect(api.sent).toEqual([
      { slug: SLUG, title: '제주 3일', value: undefined },
      { slug: SLUG, title: '제주', value: first },
    ])
    await expect(done).resolves.toBeUndefined()
  })

  it('rejects an assignment superseded by a third choice', async () => {
    const api = backend({ holds: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    const one = handle.assign(channel, first)
    await advance(0)
    // Both chosen while the first is out; only the newest is still wanted when it lands.
    const two = handle.assign(channel, second)
    const three = handle.assign(channel, third)
    const settled = Promise.all([
      expect(one).resolves.toBeUndefined(),
      expect(two).rejects.toThrow(`${noun} assignment superseded`),
      expect(three).resolves.toBeUndefined(),
    ])
    api.open()
    await advance(0)
    await settled

    // The middle choice never went out: the follow-up carries the newest one.
    expect(api.values()).toEqual([first, third])
  })

  it('rejects a pending assignment with the delete reason on discardDraftQueue', async () => {
    const api = backend({ holds: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    const done = handle.assign(channel, first)
    await advance(0)
    discardDraftQueue(SLUG)

    await expect(done).rejects.toThrow('post deleted')
  })

  it('refuses an assignment once the session has ended', async () => {
    const api = backend()
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })
    discardDraftQueues()

    const refused = expect(handle.assign(channel, first)).rejects.toThrow('session ended')
    await advance(0)
    expect(api.sent).toHaveLength(0)
    await refused
  })

  // A post published elsewhere refuses the text save that was out when the choice was made, so
  // the choice never left — and is taken back all the same (POST-86).
  it('takes back a choice made while a locked-out text save was in flight', async () => {
    const locked = new Error('POST_PUBLISHED_LOCKED')
    const api = backend({ holds: 1, failures: 1, refusal: locked })
    const handle = attach(api.send, {
      slug: SLUG,
      saved: draft('제주'),
      retry: (cause) => cause !== locked,
    })

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    const settled = expect(handle.assign(channel, first)).rejects.toBe(locked)
    api.open()
    await advance(0)
    await settled

    await advance(AUTOSAVE_RETRY_BASE_MS * 16)
    expect(api.sent).toHaveLength(1)

    handle.queue(draft('제주 4일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent[1]).toEqual({ slug: SLUG, title: '제주 4일', value: undefined })
  })
})

// Every channel's baseline is the caller's to give: one made optional would start from nothing
// and fail this at `tsc -b`.
it('takes every assignment baseline from the caller', () => {
  expectTypeOf<DraftQueueOptions['voiceId']>().toEqualTypeOf<string>()
  expectTypeOf<DraftQueueOptions['templateId']>().toEqualTypeOf<string>()
  expectTypeOf<DraftQueueOptions['targetLanguage']>().toEqualTypeOf<ContentLanguage>()
})
