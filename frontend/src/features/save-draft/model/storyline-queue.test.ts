import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AUTOSAVE_DEBOUNCE_MS } from '@/shared/config'
import {
  type Draft,
  type DraftRequest,
  type SendDraft,
  type StorylineParagraphDraft,
  attachDraftQueue,
  discardDraftQueues,
} from './draft-queue'

const STORED: StorylineParagraphDraft[] = [
  { text: '가게 앞', files: ['a.jpg'] },
  { text: '커피', files: ['b.jpg'] },
]
const SAVED: Draft = { title: '성수', memo: '', answers: [], storyline: STORED }

function recorder(options: { holds?: number } = {}) {
  let holds = options.holds ?? 0
  const requests: DraftRequest[] = []
  const held: Array<() => void> = []
  const send: SendDraft = async (request) => {
    requests.push(structuredClone(request))
    if (holds > 0) {
      holds -= 1
      await new Promise<void>((resolve) => held.push(resolve))
    }
    return request.slug
  }
  return { send, requests, open: () => held.shift()?.() }
}

function attach(send: SendDraft) {
  return attachDraftQueue({
    slug: 'p',
    saved: SAVED,
    voiceId: 'voice-a',
    templateId: '',
    targetLanguage: 'ko',
    send,
    onState: () => {},
    onMinted: () => {},
  })
}

const edited = (text: string): StorylineParagraphDraft[] => [STORED[0]!, { text, files: ['b.jpg'] }]

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  discardDraftQueues()
  vi.useRealTimers()
})

describe('the storyline field', () => {
  it('sends the newest edit, the whole list, and only while it differs from the server', async () => {
    const api = recorder()
    const handle = attach(api.send)
    handle.queue({ ...SAVED, storyline: edited('라') })
    handle.queue({ ...SAVED, storyline: edited('라떼') })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(api.requests).toHaveLength(1)
    expect(api.requests[0]!.storyline).toEqual(edited('라떼'))

    // An ordinary title save afterwards does not carry the storyline again.
    handle.queue({ ...SAVED, title: '성수동', storyline: edited('라떼') })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(api.requests).toHaveLength(2)
    expect(api.requests[1]!.storyline).toBeUndefined()
    expect(api.requests[1]!.draft.title).toBe('성수동')
  })

  it('is no edit when absent or typed back to the stored list', async () => {
    const api = recorder()
    const handle = attach(api.send)
    handle.queue({ ...SAVED, storyline: undefined })
    handle.queue({ ...SAVED, storyline: edited('라떼') })
    handle.queue({ ...SAVED, storyline: STORED.map((paragraph) => ({ ...paragraph })) })
    await vi.advanceTimersByTimeAsync(10_000)
    expect(api.requests).toHaveLength(0)
    expect(handle.state()).toBe('idle')
  })

  it('keeps an edit typed during a save, and rebases only when nothing of it is queued', async () => {
    const api = recorder({ holds: 1 })
    const handle = attach(api.send)
    handle.queue({ ...SAVED, storyline: edited('라') })
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    handle.queue({ ...SAVED, storyline: edited('라떼') })
    // A storyline job cannot replace it underneath a queued edit.
    expect(handle.rebaseStoryline([{ text: '새 스토리', files: [] }])).toBe(false)
    api.open()
    await vi.advanceTimersByTimeAsync(AUTOSAVE_DEBOUNCE_MS)
    expect(api.requests.map((request) => request.storyline)).toEqual([edited('라'), edited('라떼')])

    // Nothing queued: the server's new storyline becomes the baseline, so the old edit reads dirty.
    expect(handle.rebaseStoryline([{ text: '새 스토리', files: [] }])).toBe(true)
    handle.queue({ ...SAVED, storyline: [{ text: '새 스토리', files: [] }] })
    await vi.advanceTimersByTimeAsync(10_000)
    expect(api.requests).toHaveLength(2)
  })
})
