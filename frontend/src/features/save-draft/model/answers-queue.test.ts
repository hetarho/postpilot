import { afterEach, describe, expect, it, vi } from 'vitest'
import { attachDraftQueue, discardDraftQueue, type Draft } from './draft-queue'

const SLUG = '20260301-jeju'

function draft(overrides: Partial<Draft> = {}): Draft {
  return { title: '제주', memo: '갔다', answers: [], ...overrides }
}

/** A backend that records what each save carried. */
function backend() {
  const sent: Draft[] = []
  return {
    sent,
    send: async (_slug: string, payload: Draft) => {
      sent.push({ ...payload, answers: payload.answers.map((answer) => ({ ...answer })) })
      return SLUG
    },
  }
}

function attach(saved: Draft, send: ReturnType<typeof backend>['send']) {
  return attachDraftQueue({
    slug: SLUG,
    saved,
    voiceId: 'voice-alice',
    templateId: 'template-review',
    targetLanguage: 'ko',
    send,
    onState: () => {},
    onMinted: () => {},
  })
}

afterEach(() => {
  discardDraftQueue(SLUG)
  vi.useRealTimers()
})

describe('the data-field answers on the draft queue', () => {
  // The memo and an answer edited together are ONE save: they are the same input, and the
  // debounce is what makes typing cost one request rather than one per keystroke (POST-62).
  it('carries the memo and the answers in one save', async () => {
    vi.useFakeTimers()
    const server = backend()
    const queue = attach(draft(), server.send)

    queue.queue(
      draft({ memo: '갔다 왔다', answers: [{ label: '총평', text: '4.5점', enabled: true }] }),
    )
    await vi.runAllTimersAsync()

    expect(server.sent).toHaveLength(1)
    expect(server.sent[0].memo).toBe('갔다 왔다')
    expect(server.sent[0].answers).toEqual([{ label: '총평', text: '4.5점', enabled: true }])
  })

  it('treats a flipped switch as an edit even with the text untouched', async () => {
    vi.useFakeTimers()
    const server = backend()
    const saved = draft({ answers: [{ label: '총평', text: '4.5점', enabled: true }] })
    const queue = attach(saved, server.send)

    queue.queue(draft({ answers: [{ label: '총평', text: '4.5점', enabled: false }] }))
    await vi.runAllTimersAsync()

    expect(server.sent).toHaveLength(1)
    expect(server.sent[0].answers[0].enabled).toBe(false)
  })

  // The patch is upsert-only, so a draft carries the SELECTED template's fields and nothing
  // else. A post holding answers under another template must not read as dirty and fire a save
  // nobody asked for.
  it('stands down when the fields on screen already match the server', async () => {
    vi.useFakeTimers()
    const server = backend()
    const queue = attach(
      draft({ answers: [{ label: '다른 칸', text: '값', enabled: true }] }),
      server.send,
    )

    // No template selected: no fields, so nothing to send.
    queue.queue(draft())
    // A template whose one field nobody has answered: the default the screen renders is not an
    // edit either.
    queue.queue(draft({ answers: [{ label: '총평', text: '', enabled: true }] }))
    await vi.runAllTimersAsync()

    expect(server.sent).toEqual([])
    expect(queue.state()).toBe('idle')
  })

  // On /posts/new nothing is saved until there is something to save, and then the answers ride
  // the create that mints the slug — the same rule the title and the memo follow.
  it('carries the answers into the create that mints the post', async () => {
    const server = backend()
    const queue = attachDraftQueue({
      slug: undefined,
      saved: draft({ title: '', memo: '' }),
      voiceId: 'voice-alice',
      templateId: 'template-review',
      targetLanguage: 'ko',
      send: server.send,
      onState: () => {},
      onMinted: () => {},
    })

    queue.queue(draft({ answers: [{ label: '총평', text: '4.5점', enabled: true }] }))
    expect(server.sent).toEqual([])
    await queue.flush()

    expect(server.sent).toHaveLength(1)
    expect(server.sent[0].answers).toEqual([{ label: '총평', text: '4.5점', enabled: true }])
  })

  it('waits for a queued answer before flush resolves', async () => {
    const server = backend()
    const queue = attach(draft(), server.send)

    queue.queue(draft({ answers: [{ label: '총평', text: '4.5점', enabled: true }] }))
    await queue.flush()

    expect(server.sent).toHaveLength(1)
    expect(server.sent[0].answers[0].text).toBe('4.5점')
  })

  // The baseline accumulates what the server holds: replacing it outright would drop the labels
  // a save did not carry, and the editor would re-send them the next time they were on screen.
  it('keeps a label the next save does not carry out of the dirty check', async () => {
    vi.useFakeTimers()
    const server = backend()
    const queue = attach(draft(), server.send)

    queue.queue(draft({ answers: [{ label: '총평', text: '4.5점', enabled: true }] }))
    await vi.runAllTimersAsync()
    // A different template's field is answered next; 총평 is off screen.
    queue.queue(draft({ answers: [{ label: '방문일', text: '2026-03-01', enabled: true }] }))
    await vi.runAllTimersAsync()
    // Back to the first template, holding what the server already has.
    queue.queue(draft({ answers: [{ label: '총평', text: '4.5점', enabled: true }] }))
    await vi.runAllTimersAsync()

    expect(server.sent).toHaveLength(2)
  })
})
