import { StrictMode } from 'react'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { spokenVoiceFixture } from '@/test/spoken-voices'
import { createTestQueryClient } from '@/test/session'
import { SpokenSamplePlayer } from './SpokenSamplePlayer'

let element: HTMLMediaElement | undefined
beforeEach(() => {
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(() => {
    element = document.querySelector<HTMLMediaElement>('audio[src]') ?? undefined
    return Promise.resolve()
  })
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.stubGlobal(
    'fetch',
    vi
      .fn()
      .mockResolvedValue(
        new Response(new Blob(['mp3']), { headers: { 'content-type': 'audio/mpeg' } }),
      ),
  )
  URL.createObjectURL = vi.fn(() => 'blob:private')
  URL.revokeObjectURL = vi.fn()
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  element = undefined
})

it('starts on keyboard gesture, acknowledges only playing, replays locally and revokes private audio on account change', async () => {
  const f = spokenVoiceFixture(),
    client = createTestQueryClient(),
    played = vi.fn().mockResolvedValue(undefined),
    user = userEvent.setup()
  const view = (ownerId: string) => (
    <StrictMode>
      <TransportProvider transport={f.transport}>
        <QueryClientProvider client={client}>
          <SpokenSamplePlayer
            ownerId={ownerId}
            assetId={'4'.repeat(32)}
            name="Sample"
            durationMs={1500}
            onPlayed={played}
          />
        </QueryClientProvider>
      </TransportProvider>
    </StrictMode>
  )
  const mounted = render(view('alice'))
  expect(f.calls).toEqual([])
  await user.tab()
  await user.keyboard('{Enter}')
  await waitFor(() => expect(element).toBeDefined())
  expect(played).not.toHaveBeenCalled()
  fireEvent.playing(element!)
  expect(played).toHaveBeenCalledOnce()
  fireEvent.ended(element!)
  await user.keyboard('{Enter}')
  await waitFor(() => expect(HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(2))
  expect(f.calls).toEqual(['sample'])
  const signal = vi.mocked(fetch).mock.calls[0]?.[1]?.signal
  mounted.rerender(view('bob'))
  expect(signal?.aborted).toBe(true)
  expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:private')
  expect(played).toHaveBeenCalledOnce()
})

it('abandons an in-flight private download without playback or acknowledgment on unmount', async () => {
  const f = spokenVoiceFixture(),
    played = vi.fn(),
    user = userEvent.setup()
  let finish: ((response: Response) => void) | undefined
  vi.mocked(fetch).mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve
      }),
  )
  const mounted = render(
    <TransportProvider transport={f.transport}>
      <QueryClientProvider client={createTestQueryClient()}>
        <SpokenSamplePlayer
          ownerId="alice"
          assetId={'4'.repeat(32)}
          name="Sample"
          durationMs={1500}
          onPlayed={played}
        />
      </QueryClientProvider>
    </TransportProvider>,
  )
  await user.tab()
  await user.keyboard('{Enter}')
  await waitFor(() => expect(fetch).toHaveBeenCalledOnce())
  const signal = vi.mocked(fetch).mock.calls[0]?.[1]?.signal
  mounted.unmount()
  finish?.(new Response(new Blob(['mp3']), { headers: { 'content-type': 'audio/mpeg' } }))
  await waitFor(() => expect(signal?.aborted).toBe(true))
  expect(HTMLMediaElement.prototype.play).not.toHaveBeenCalled()
  expect(played).not.toHaveBeenCalled()
  expect(URL.createObjectURL).not.toHaveBeenCalled()
})
