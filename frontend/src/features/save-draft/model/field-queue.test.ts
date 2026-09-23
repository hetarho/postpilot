import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AUTOSAVE_DEBOUNCE_MS, AUTOSAVE_RETRY_BASE_MS } from '@/shared/config'
import {
  type Draft,
  type SendDraft,
  attachDraftQueue,
  discardDraftQueue,
  discardDraftQueues,
} from './draft-queue'

const SLUG = '20260301-jeju'

function draft(title: string): Draft {
  return { title, memo: '', answers: [] }
}

/** A backend that records the title and the 분야 each save carried. Its failures and timing are the
 *  test's to decide. */
function backend(
  options: { failures?: number; holds?: number; mint?: string; refusal?: Error } = {},
) {
  let failures = options.failures ?? 0
  let holds = options.holds ?? 0
  const sent: Array<{ slug: string; title: string; field: string | undefined }> = []
  const held: Array<() => void> = []
  const send: SendDraft = async (slug, value, _voiceId, _templateId, _targetLanguage, field) => {
    sent.push({ slug, title: value.title, field })
    if (holds > 0) {
      holds -= 1
      await new Promise<void>((resolve) => held.push(resolve))
    }
    if (failures > 0) {
      failures -= 1
      throw options.refusal ?? new Error('offline')
    }
    return slug || (options.mint ?? '20260828-minted')
  }
  return {
    send,
    sent,
    fields: () => sent.map((call) => call.field),
    /** Lets the oldest held request finish. */
    open: () => held.shift()?.(),
  }
}

function attach(
  send: SendDraft,
  options: {
    slug?: string
    saved?: Draft
    fieldId?: string
    retry?: (cause: unknown) => boolean
  } = {},
) {
  return attachDraftQueue({
    slug: options.slug,
    saved: options.saved ?? draft(''),
    voiceId: 'voice-a',
    templateId: '',
    fieldId: options.fieldId,
    targetLanguage: 'ko',
    send,
    retry: options.retry,
    onState: () => {},
    onMinted: () => {},
  })
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

// POST-82: the 분야 is the 템플릿's mechanism exactly — it rides the queue as its own assignment,
// and '' (없음) is a real value that clears.
describe('the post 분야', () => {
  it('sends no field on a create that stayed on 없음', async () => {
    const api = backend()
    const handle = attach(api.send)

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    // Omitted, not '': a create has nothing to clear.
    expect(api.fields()).toEqual([undefined])
  })

  it('carries a 분야 chosen before the post exists into the create', async () => {
    const api = backend()
    const handle = attach(api.send)

    await expect(handle.assignField('cafe')).resolves.toBeUndefined()
    await advance(10_000)
    expect(api.sent).toHaveLength(0)

    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.fields()).toEqual(['cafe'])

    // The create settled it: the next save is text alone.
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.fields()).toEqual(['cafe', undefined])
  })

  // A choice is not text: picked, typed and erased, a new draft still has nothing to create.
  it('creates no post for a 분야 alone', async () => {
    const api = backend()
    const handle = attach(api.send)

    await handle.assignField('cafe')
    handle.queue(draft('제'))
    handle.queue(draft(''))
    await advance(10_000)

    expect(api.sent).toHaveLength(0)
  })

  // The 템플릿's rule: a create is retried whole, since there is no post yet to take a choice back to.
  it('keeps a 분야 on a refused create and retries with it', async () => {
    const api = backend({ failures: 1 })
    const handle = attach(api.send)

    await handle.assignField('cafe')
    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    await advance(AUTOSAVE_RETRY_BASE_MS)

    expect(api.fields()).toEqual(['cafe', 'cafe'])
  })

  // The choice landed during the create round trip, so the create carried the old one.
  it('follows a 분야 chosen while the create was out with an immediate update', async () => {
    const api = backend({ holds: 1, mint: '20260828-제주' })
    const handle = attach(api.send)
    handle.queue(draft('제주'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.fields()).toEqual([undefined])

    await handle.assignField('cafe')
    api.open()
    // No debounce: an assignment is an action, not a keystroke.
    await advance(0)

    expect(api.sent[1]).toEqual({ slug: '20260828-제주', title: '제주', field: 'cafe' })
    expect(handle.state()).toBe('saved')
  })

  it('assigns an existing post at once, then leaves later saves alone', async () => {
    const api = backend()
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    const done = handle.assignField('cafe')
    await advance(0)
    await expect(done).resolves.toBeUndefined()
    expect(api.sent).toEqual([{ slug: SLUG, title: '제주', field: 'cafe' }])

    // The value the server now holds is no change at all.
    await handle.assignField('cafe')
    await advance(0)
    expect(api.sent).toHaveLength(1)

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.fields()).toEqual(['cafe', undefined])
  })

  it('leaves an existing post’s 분야 off an ordinary text save', async () => {
    const api = backend()
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주'), fieldId: 'cafe' })

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.sent).toEqual([{ slug: SLUG, title: '제주 3일', field: undefined }])
  })

  it('sends an empty string to clear an assignment', async () => {
    const api = backend()
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주'), fieldId: 'cafe' })

    // What the post already holds, as the editor was told: nothing to send.
    await handle.assignField('cafe')
    await advance(0)
    expect(api.sent).toHaveLength(0)

    await handle.assignField('')
    await advance(0)

    // Present-and-empty, which the editor sends as UNSPECIFIED: the clear, distinct from omitting.
    expect(api.fields()).toEqual([''])
  })

  // The bug the queue exists to prevent: a title save that left before the choice must not carry
  // the old 분야 back over it.
  it('does not let text typed during an assignment revert it', async () => {
    const api = backend({ holds: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    const done = handle.assignField('cafe')
    await advance(0)
    handle.queue(draft('제주 3일'))
    api.open()
    await done
    await advance(AUTOSAVE_DEBOUNCE_MS)

    expect(api.sent).toEqual([
      { slug: SLUG, title: '제주', field: 'cafe' },
      { slug: SLUG, title: '제주 3일', field: undefined },
    ])
  })

  it('takes a refused assignment back so the next save carries text only', async () => {
    const api = backend({ failures: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    await expect(handle.assignField('cafe')).rejects.toThrow('offline')
    // Nothing else changed, so nothing is retried.
    await advance(AUTOSAVE_RETRY_BASE_MS * 4)
    expect(api.sent).toHaveLength(1)

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent[1]).toEqual({ slug: SLUG, title: '제주 3일', field: undefined })
  })

  // The text typed back has nothing left to say, but a 분야 chosen behind the failing save still
  // does, so the retry is not stood down.
  it('retries a 분야 chosen behind a failing text save typed back meanwhile', async () => {
    const api = backend({ holds: 1, failures: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    const done = handle.assignField('cafe')
    handle.queue(draft('제주'))
    api.open()
    await advance(AUTOSAVE_RETRY_BASE_MS)

    expect(api.sent).toEqual([
      { slug: SLUG, title: '제주 3일', field: undefined },
      { slug: SLUG, title: '제주', field: 'cafe' },
    ])
    await expect(done).resolves.toBeUndefined()
  })

  it('keeps a 분야 waiting through a backoff when the text is typed back', async () => {
    const api = backend({ holds: 1, failures: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })
    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)

    const done = handle.assignField('cafe')
    api.open()
    // The text save failed and a retry is waiting; the text goes back to what the server holds.
    await advance(0)
    handle.queue(draft('제주'))
    await advance(AUTOSAVE_RETRY_BASE_MS)

    expect(api.sent).toEqual([
      { slug: SLUG, title: '제주 3일', field: undefined },
      { slug: SLUG, title: '제주', field: 'cafe' },
    ])
    await expect(done).resolves.toBeUndefined()
  })

  it('rejects an assignment superseded by a third choice', async () => {
    const api = backend({ holds: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    const first = handle.assignField('cafe')
    await advance(0)
    // Both chosen while the first is out; only the newest is still wanted when it lands.
    const second = handle.assignField('pets')
    const third = handle.assignField('restaurant')
    const settled = Promise.all([
      expect(first).resolves.toBeUndefined(),
      expect(second).rejects.toThrow('field assignment superseded'),
      expect(third).resolves.toBeUndefined(),
    ])
    api.open()
    await advance(0)
    await settled

    // The middle choice never went out: the follow-up carries the newest one.
    expect(api.fields()).toEqual(['cafe', 'restaurant'])
  })

  it('rejects a pending assignment with the delete reason on discardDraftQueue', async () => {
    const api = backend({ holds: 1 })
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })

    const done = handle.assignField('cafe')
    await advance(0)
    discardDraftQueue(SLUG)

    await expect(done).rejects.toThrow('post deleted')
  })

  it('refuses an assignment once the session has ended', async () => {
    const api = backend()
    const handle = attach(api.send, { slug: SLUG, saved: draft('제주') })
    discardDraftQueues()

    const refused = expect(handle.assignField('cafe')).rejects.toThrow('session ended')
    await advance(0)
    expect(api.sent).toHaveLength(0)
    await refused
  })

  // T339's answer-refusal: a post published elsewhere refuses the text save that was out when the
  // 분야 was chosen, so the choice never left — and is taken back all the same (POST-86).
  it('takes back a 분야 chosen while a locked-out text save was in flight', async () => {
    const locked = new Error('POST_PUBLISHED_LOCKED')
    const api = backend({ holds: 1, failures: 1, refusal: locked })
    const handle = attach(api.send, {
      slug: SLUG,
      saved: draft('제주'),
      retry: (cause) => cause !== locked,
    })

    handle.queue(draft('제주 3일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    const settled = expect(handle.assignField('cafe')).rejects.toBe(locked)
    api.open()
    await advance(0)
    await settled

    await advance(AUTOSAVE_RETRY_BASE_MS * 16)
    expect(api.sent).toHaveLength(1)

    handle.queue(draft('제주 4일'))
    await advance(AUTOSAVE_DEBOUNCE_MS)
    expect(api.sent[1]).toEqual({ slug: SLUG, title: '제주 4일', field: undefined })
  })
})
