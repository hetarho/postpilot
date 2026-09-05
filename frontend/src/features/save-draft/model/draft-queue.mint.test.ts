import { afterEach, describe, expect, it, vi } from 'vitest'
import { attachDraftQueue, discardDraftQueues } from './draft-queue'

afterEach(() => {
  discardDraftQueues()
})

describe('mint', () => {
  it('creates the post from the empty draft when nothing has been typed yet', async () => {
    const send = vi.fn(async () => '20260828-untitled')
    const onMinted = vi.fn()
    const handle = attachDraftQueue({
      slug: undefined,
      saved: { title: '', memo: '' },
      voiceId: 'voice-a',
      templateId: '',
      targetLanguage: 'ko',
      send,
      onState: () => {},
      onMinted,
    })

    await expect(handle.mint()).resolves.toBe('20260828-untitled')
    // The create names its voice even though nothing else was typed (spec/legacy/policy/posts.md).
    expect(send).toHaveBeenCalledWith('', { title: '', memo: '' }, 'voice-a', undefined, 'ko')
    expect(onMinted).toHaveBeenCalledWith('20260828-untitled')
  })

  it('sends the text already queued rather than an empty draft, without waiting for the debounce', async () => {
    const send = vi.fn(async () => '20260828-jeju')
    const handle = attachDraftQueue({
      slug: undefined,
      saved: { title: '', memo: '' },
      voiceId: 'voice-a',
      templateId: '',
      targetLanguage: 'ko',
      send,
      onState: () => {},
      onMinted: () => {},
    })
    handle.queue({ title: '제주', memo: '' })

    await expect(handle.mint()).resolves.toBe('20260828-jeju')
    expect(send).toHaveBeenCalledTimes(1)
    expect(send).toHaveBeenCalledWith('', { title: '제주', memo: '' }, 'voice-a', undefined, 'ko')
  })

  // An empty draft equals what the server "holds" for a new post, so without care a
  // failed create would be mistaken for "typed back to saved" and never retried.
  it('keeps retrying a failed create of the empty draft until the post exists', async () => {
    vi.useFakeTimers()
    try {
      const send = vi
        .fn<() => Promise<string>>()
        .mockRejectedValueOnce(new Error('unavailable'))
        .mockResolvedValueOnce('20260828-untitled')
      const handle = attachDraftQueue({
        slug: undefined,
        saved: { title: '', memo: '' },
        voiceId: 'voice-a',
        templateId: '',
        targetLanguage: 'ko',
        send,
        onState: () => {},
        onMinted: () => {},
      })
      const minted = handle.mint()

      await vi.advanceTimersByTimeAsync(1_000)
      await vi.advanceTimersByTimeAsync(1_000)

      await expect(minted).resolves.toBe('20260828-untitled')
      expect(send).toHaveBeenCalledTimes(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('answers immediately for a post that already has a slug', async () => {
    const send = vi.fn(async () => 'never')
    const handle = attachDraftQueue({
      slug: '20260828-jeju',
      saved: { title: '제주', memo: '' },
      voiceId: 'voice-a',
      templateId: '',
      targetLanguage: 'ko',
      send,
      onState: () => {},
      onMinted: () => {},
    })

    await expect(handle.mint()).resolves.toBe('20260828-jeju')
    expect(send).not.toHaveBeenCalled()
  })

  it('rejects at once on a queue whose session has already ended', async () => {
    const handle = attachDraftQueue({
      slug: undefined,
      saved: { title: '', memo: '' },
      voiceId: 'voice-a',
      templateId: '',
      targetLanguage: 'ko',
      send: vi.fn(async () => 'never'),
      onState: () => {},
      onMinted: () => {},
    })
    discardDraftQueues()

    await expect(handle.mint()).rejects.toThrow('session ended')
  })

  it('rejects when the session ends before the post exists', async () => {
    const handle = attachDraftQueue({
      slug: undefined,
      saved: { title: '', memo: '' },
      voiceId: 'voice-a',
      templateId: '',
      targetLanguage: 'ko',
      send: () => new Promise(() => {}),
      onState: () => {},
      onMinted: () => {},
    })
    const minted = handle.mint()

    discardDraftQueues()

    await expect(minted).rejects.toThrow('session ended')
  })
})
