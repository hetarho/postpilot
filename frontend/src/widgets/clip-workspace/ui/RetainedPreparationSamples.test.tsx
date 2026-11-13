import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { SpokenSamplePlayer } from '@/entities/spoken-voice'
import type { ClipPreparationScreen } from '@/features/edit-clip-project'
import { spokenVoiceFixture } from '@/test/spoken-voices'
import { createTestQueryClient } from '@/test/session'
import { ClipPreparationGuide } from './ClipPreparationGuide'
import { RetainedPreparationMedia } from './RetainedPreparationMedia'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}
function sampleResponse() {
  return new Response(new Blob(['mp3']), { headers: { 'content-type': 'audio/mpeg' } })
}
function startPlayback(media: HTMLMediaElement) {
  Object.defineProperty(media, 'paused', { configurable: true, value: false })
  fireEvent.play(media)
  if (!media.paused) fireEvent.playing(media)
}

beforeEach(() => {
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(function (
    this: HTMLMediaElement,
  ) {
    startPlayback(this)
    // Like a browser, a pause before playback starts rejects the play request.
    return this.paused
      ? Promise.reject(new DOMException('The play() request was interrupted.', 'AbortError'))
      : Promise.resolve()
  })
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(function (
    this: HTMLMediaElement,
  ) {
    Object.defineProperty(this, 'paused', { configurable: true, value: true })
    fireEvent.pause(this)
  })
  vi.stubGlobal(
    'fetch',
    vi.fn().mockImplementation(async () => sampleResponse()),
  )
  vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:sample')
  vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function fixture() {
  const transport = spokenVoiceFixture().transport
  const client = createTestQueryClient()
  const acknowledged = vi.fn().mockResolvedValue(undefined)
  const failed = vi.fn()
  function View({
    selected = 'style',
    lifecycle = false,
  }: {
    selected?: ClipPreparationScreen
    lifecycle?: boolean
  }) {
    return (
      <TransportProvider transport={transport}>
        <QueryClientProvider client={client}>
          <div hidden={lifecycle}>
            <ClipPreparationGuide
              screen={selected}
              onSelect={() => {}}
              campaign={false}
              noteEmpty
              screens={{
                kind: null,
                requirements: null,
                template: null,
                note: null,
                sources: <input aria-label="Upload" type="file" />,
                content: <input aria-label="Title" defaultValue="Retained title" />,
                style: (
                  <RetainedPreparationMedia active={!lifecycle && selected === 'style'}>
                    <input aria-label="Duration" defaultValue="42" />
                    <select aria-label="Voice" defaultValue="confirmed">
                      <option value="confirmed">Confirmed voice</option>
                    </select>
                    <SpokenSamplePlayer
                      ownerId="alice"
                      assetId={'4'.repeat(32)}
                      name="Sample"
                      durationMs={1500}
                      onPlayed={acknowledged}
                      onFailure={failed}
                    />
                  </RetainedPreparationMedia>
                ),
              }}
              rows={[]}
              models={null}
            />
          </div>
        </QueryClientProvider>
      </TransportProvider>
    )
  }
  return { View, acknowledged, failed }
}

it.each(['sources', 'review', 'edit', 'running'] as const)(
  'stops style samples on %s, retains controls/uploads and permits explicit visible replay',
  async (destination) => {
    const { View, acknowledged } = fixture()
    const view = render(<View />)
    const duration = screen.getByLabelText('Duration')
    const voice = screen.getByLabelText('Voice')
    const upload = screen.getByLabelText('Upload') as HTMLInputElement
    const file = new File(['source'], 'source.mp4', { type: 'video/mp4' })
    fireEvent.change(upload, { target: { files: [file] } })
    fireEvent.change(duration, { target: { value: '51' } })
    fireEvent.click(screen.getByRole('button', { name: /Sample/ }))
    await waitFor(() => expect(acknowledged).toHaveBeenCalledOnce())
    const audio = view.container.querySelector('audio')!
    expect(audio.paused).toBe(false)
    const props =
      destination === 'sources' || destination === 'review'
        ? { selected: destination }
        : { lifecycle: true }
    view.rerender(<View {...props} />)
    expect(audio.paused).toBe(true)
    expect(audio).not.toBeVisible()
    view.rerender(<View />)
    expect(view.container.querySelector('audio')).toBe(audio)
    expect(screen.getByLabelText('Duration')).toBe(duration)
    expect(duration).toHaveValue('51')
    expect(screen.getByLabelText('Voice')).toBe(voice)
    expect(voice).toHaveValue('confirmed')
    expect(upload.files?.[0]).toBe(file)
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalledOnce()
    fireEvent.click(screen.getByRole('button', { name: /Sample/ }))
    await waitFor(() => expect(HTMLMediaElement.prototype.play).toHaveBeenCalledTimes(2))
    expect(audio.paused).toBe(false)
  },
)

it.each(['download', 'play'] as const)(
  'blocks a sample %s completing after style is hidden, including a late playing event',
  async (stage) => {
    const download = deferred<Response>()
    const playback = deferred<void>()
    if (stage === 'download') vi.mocked(fetch).mockReturnValue(download.promise)
    if (stage === 'play')
      vi.mocked(HTMLMediaElement.prototype.play).mockImplementation(function (
        this: HTMLMediaElement,
      ) {
        return playback.promise.then(() => startPlayback(this))
      })
    const { View, acknowledged, failed } = fixture()
    const view = render(<View />)
    fireEvent.click(screen.getByRole('button', { name: /Sample/ }))
    await waitFor(() => expect(fetch).toHaveBeenCalledOnce())
    if (stage === 'play')
      await waitFor(() => expect(HTMLMediaElement.prototype.play).toHaveBeenCalledOnce())
    view.rerender(<View selected="review" />)
    await act(async () => {
      download.resolve(sampleResponse())
      playback.resolve()
    })
    await waitFor(() => expect(HTMLMediaElement.prototype.play).toHaveBeenCalledOnce())
    const audio = view.container.querySelector('audio')!
    expect(audio.paused).toBe(true)
    // A queued native playing event must also be stopped before the player's acknowledgment.
    fireEvent.playing(audio)
    expect(audio.paused).toBe(true)
    expect(acknowledged).not.toHaveBeenCalled()
    expect(failed).not.toHaveBeenCalled()
    view.rerender(<View />)
    expect(screen.getByRole('button', { name: /Sample/ })).toBeEnabled()
  },
)
