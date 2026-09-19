import type { ComponentProps } from 'react'
import { Code } from '@connectrpc/connect'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { connectAppError } from '@/test/app-error'
import type { ClipEditPlan } from '@/entities/clip-plan'
import type { ClipPreviewOverlay } from '../model/draft-preview'
import { ClipDraftPreview } from './ClipDraftPreview'

/** The prepared overlay as `features/preview-clip-draft` hands it over: this component fetches
 *  nothing, so the test states the overlay instead of standing up a fake service. */
const overlay = (patch: Partial<ClipPreviewOverlay> = {}): ClipPreviewOverlay => ({
  assets: [
    {
      key: 'glyph',
      instanceId: 'fixed',
      url: 'blob:glyph',
      x: 0,
      y: 0,
      width: 1,
      height: 1,
      startMs: 0,
      endMs: 19800,
      inMs: 0,
      outMs: 0,
      dy: 0,
      layer: 0,
    },
  ],
  canvasWidth: 1080,
  canvasHeight: 1920,
  ready: true,
  updating: false,
  onRetry: vi.fn(),
  ...patch,
})

const draft: ClipEditPlan = {
  durationMs: 19800,
  hook: '',
  cuts: [
    {
      id: 'a',
      sourceId: 'a',
      fingerprint: 'a',
      startMs: 2000,
      endMs: 12000,
      transitionMs: 0,
      copies: [],
      chips: [],
      volumePermille: 1000,
      playbackRatePermille: 1000,
      focal: { x: 0, y: 0.5 },
    },
    {
      id: 'b',
      sourceId: 'b',
      fingerprint: 'b',
      startMs: 3000,
      endMs: 13000,
      transitionMs: 200,
      copies: [],
      chips: [],
      volumePermille: 500,
      playbackRatePermille: 1000,
    },
  ],
}
const sources = draft.cuts.map((c) => ({
  id: c.id,
  fingerprint: c.fingerprint,
  filename: `${c.id}.mp4`,
  durationMs: 30000,
  width: 1920,
  height: 1080,
  allowedRatePermille: [500, 750, 1000, 1250, 1500, 2000],
}))
beforeEach(() => {
  vi.stubGlobal('MediaError', { MEDIA_ERR_SRC_NOT_SUPPORTED: 4, MEDIA_ERR_DECODE: 3 })
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = vi.fn(() => 'blob:glyph')
      static revokeObjectURL = vi.fn()
    },
  )
})
afterEach(() => {
  cleanup()
  Reflect.deleteProperty(HTMLVideoElement.prototype, 'requestVideoFrameCallback')
  Reflect.deleteProperty(HTMLVideoElement.prototype, 'cancelVideoFrameCallback')
  Reflect.deleteProperty(HTMLMediaElement.prototype, 'preservesPitch')
  Reflect.deleteProperty(HTMLMediaElement.prototype, 'webkitPreservesPitch')
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function mount(
  ratio = 'vertical',
  access = vi.fn(async (fp: string, refresh?: boolean) => {
    void refresh
    return `blob:${fp}`
  }),
  props: Partial<ComponentProps<typeof ClipDraftPreview>> = {},
) {
  const preview = props.preview ?? overlay()
  const view = render(
    <ClipDraftPreview
      plan={draft}
      ratio={ratio}
      sources={sources}
      resolvePlayback={access}
      {...props}
      preview={preview}
    />,
  )
  return { ...view, access, preview }
}

// The players render before their signed URLs resolve, and the sync effect that attaches the
// loadedmetadata listener runs only after `src` commits. Waiting on the element count alone
// races both steps, so callers name a property the effect writes and we wait until it lands.
async function syncedPlayers(
  container: HTMLElement,
  count: number,
  synced: (player: HTMLVideoElement, index: number) => void,
) {
  await waitFor(() => {
    const players = [...container.querySelectorAll('video')]
    expect(players).toHaveLength(count)
    players.forEach((player, index) => {
      expect(player).toHaveAttribute('src')
      synced(player, index)
    })
  })
  return [...container.querySelectorAll('video')]
}

it.each([500, 750, 1000, 1250, 1500, 2000])(
  'sets native rate and supported pitch preservation at %i without granting audio permission',
  async (rate) => {
    const enabled = {
      ...draft,
      cuts: draft.cuts.map((c) => ({ ...c, playbackRatePermille: rate })),
      sourceAudio: draft.cuts.map((c, i) => ({
        sourceId: c.sourceId,
        fingerprint: c.fingerprint,
        retainOriginalAudio: i === 1,
      })),
    }
    Object.defineProperty(HTMLMediaElement.prototype, 'preservesPitch', {
      configurable: true,
      writable: true,
      value: false,
    })
    Object.defineProperty(HTMLMediaElement.prototype, 'webkitPreservesPitch', {
      configurable: true,
      writable: true,
      value: false,
    })
    const view = mount('vertical', undefined, { plan: enabled, timeMs: 1000 })
    const [off, on] = await syncedPlayers(view.container, 2, (player) =>
      expect(player.preservesPitch).toBe(true),
    )
    Object.defineProperty(off, 'readyState', { value: 4 })
    fireEvent.loadedMetadata(off)
    expect(off.currentTime).toBe(2 + rate / 1000)
    expect(off.playbackRate).toBe(rate / 1000)
    expect(off.preservesPitch).toBe(true)
    expect((off as HTMLVideoElement & { webkitPreservesPitch: boolean }).webkitPreservesPitch).toBe(
      true,
    )
    // The sound control stands on the frame, as a player's does, and says which way it is set.
    const sound = screen.getByRole('button', { name: '미리보기 소리 듣기' })
    expect(sound).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(sound)
    expect(sound).toHaveAttribute('aria-pressed', 'true')
    expect(off.muted).toBe(true)
    expect(on.muted).toBe(false)
    expect(off.volume).toBe(1)
    expect(on.volume).toBe(0) // preloaded neighbour has no transition gain yet
    expect(enabled.sourceAudio[0].retainOriginalAudio).toBe(false)
  },
)

it('corrects transition-player drift on the transformed clock and reports inverse source time', async () => {
  const frames = new Map<HTMLVideoElement, VideoFrameRequestCallback>()
  Object.defineProperty(HTMLVideoElement.prototype, 'requestVideoFrameCallback', {
    configurable: true,
    value: function (this: HTMLVideoElement, callback: VideoFrameRequestCallback) {
      frames.set(this, callback)
      return 1
    },
  })
  Object.defineProperty(HTMLVideoElement.prototype, 'cancelVideoFrameCallback', {
    configurable: true,
    value: () => {},
  })
  const onDisplayedFrame = vi.fn()
  const plan = {
    ...draft,
    cuts: draft.cuts.map((c, i) => ({ ...c, playbackRatePermille: i ? 2000 : 500 })),
    sourceAudio: draft.cuts.map((c) => ({
      sourceId: c.sourceId,
      fingerprint: c.fingerprint,
      retainOriginalAudio: true,
    })),
  }
  const view = mount('vertical', undefined, { plan, timeMs: 19900, onDisplayedFrame })
  const [left, right] = await syncedPlayers(view.container, 2, (player, i) =>
    expect(player.playbackRate).toBe(i ? 2 : 0.5),
  )
  for (const video of [left, right]) {
    Object.defineProperty(video, 'readyState', { value: 4 })
    video.currentTime = 0
    fireEvent.loadedMetadata(video)
  }
  expect(left.currentTime).toBe(11.95)
  expect(right.currentTime).toBe(3.2)
  expect(left.volume).toBe(0.5)
  expect(right.volume).toBe(0.25)
  act(() => frames.get(right)!(0, { mediaTime: 3.2 } as VideoFrameCallbackMetadata))
  expect(onDisplayedFrame).toHaveBeenCalledWith({
    cutId: 'b',
    sourceMs: 3200,
    outputMs: 19900,
    precise: true,
  })
})

it.each(['vertical', 'horizontal', 'square'])(
  'prepares %s current draft overlays with at most two source players',
  async (ratio) => {
    const view = mount(ratio)
    await screen.findByText('현재 편집 내용의 미리보기예요.')
    expect(view.container.querySelectorAll('video')).toHaveLength(2)
    expect(view.container.querySelectorAll('img')).toHaveLength(1)
    expect(view.access).toHaveBeenCalledWith('a')
    expect(view.access).toHaveBeenCalledWith('b')
  },
)

it('keeps the missing original state separate from successful text preparation', async () => {
  mount(
    'vertical',
    vi.fn(async () => {
      throw connectAppError('CLIP_SOURCE_EXPIRED', Code.FailedPrecondition)
    }),
  )
  expect(
    await screen.findByText(
      '원본 보관 시간이 지났어요. 같은 원본을 다시 선택하면 미리 볼 수 있어요.',
    ),
  ).toBeVisible()
  expect(await screen.findByText('현재 편집 내용의 미리보기예요.')).toBeVisible()
})

it('refreshes a failed signed URL once and keeps codec failure explicit', async () => {
  const view = mount()
  await waitFor(() => expect(view.container.querySelectorAll('video')).toHaveLength(2))
  const first = view.container.querySelector('video')!
  fireEvent.error(first)
  await waitFor(() => expect(view.access).toHaveBeenCalledWith('a', true))
  fireEvent.error(first)
  expect(
    await screen.findByText('원본을 재생하지 못했어요. 글과 시간은 계속 수정할 수 있어요.'),
  ).toBeVisible()
  expect(view.access.mock.calls.filter(([, refresh]) => refresh)).toHaveLength(1)
  view.unmount()
  const next = mount()
  await waitFor(() => expect(next.container.querySelector('video')).not.toBeNull())
  const codec = next.container.querySelector('video')!
  Object.defineProperty(codec, 'error', { value: { code: 4 } })
  fireEvent.error(codec)
  expect(
    await screen.findByText(
      '이 브라우저에서는 원본 형식을 재생할 수 없어요. 글과 시간은 계속 수정할 수 있어요.',
    ),
  ).toBeVisible()
})

/** Frame callbacks the test fires by hand, and a pitch flag the sync effect flips so a test can
 *  wait for the effect to have run against each player. */
function installFrameCallbacks() {
  const frames = new Map<HTMLVideoElement, VideoFrameRequestCallback>()
  Object.defineProperty(HTMLVideoElement.prototype, 'requestVideoFrameCallback', {
    configurable: true,
    value: function (this: HTMLVideoElement, callback: VideoFrameRequestCallback) {
      frames.set(this, callback)
      return 1
    },
  })
  Object.defineProperty(HTMLVideoElement.prototype, 'cancelVideoFrameCallback', {
    configurable: true,
    value: () => {},
  })
  Object.defineProperty(HTMLMediaElement.prototype, 'preservesPitch', {
    configurable: true,
    writable: true,
    value: false,
  })
  return frames
}
const synced = (player: HTMLVideoElement) => expect(player.preservesPitch).toBe(true)
function loaded(...players: HTMLVideoElement[]) {
  for (const player of players) {
    Object.defineProperty(player, 'readyState', { value: 4 })
    fireEvent.loadedMetadata(player)
  }
}
const scrubber = () => screen.getByRole('slider', { name: '완성 영상 기준 시간' })

// A cut that runs to the end of its source never presents a frame at or past the cut's end: the
// element stops at EOF with the clock a frame short, and the next cut was never handed the
// playhead — the preview played cut 1 and stopped there.
it("hands playback to the next cut when the current cut's footage ends", async () => {
  installFrameCallbacks()
  const play = vi.mocked(HTMLMediaElement.prototype.play)
  const view = mount()
  const [a, b] = await syncedPlayers(view.container, 2, synced)
  loaded(a!, b!)
  expect(a!.currentTime).toBe(2)
  expect(b!.currentTime).toBe(3)
  fireEvent.click(screen.getByRole('button', { name: '재생' }))
  await waitFor(() => expect(play.mock.contexts).toContain(a))
  expect(play.mock.contexts).not.toContain(b)
  fireEvent(a!, new Event('ended'))
  expect(scrubber()).toHaveAttribute('aria-valuetext', '10.000 / 19.800 s')
  await waitFor(() => expect(play.mock.contexts).toContain(b))
  expect(screen.getByRole('button', { name: '일시 정지' })).toBeInTheDocument()
})

// The playing master is the clock: seeking it back to the time it reported a render ago made the
// picture hop backwards every few frames after a scrub. Paused, the requested position is the
// truth again.
it('never seeks the playing master back to the time it last reported', async () => {
  const frames = installFrameCallbacks()
  const view = mount()
  const [a] = await syncedPlayers(view.container, 2, synced)
  loaded(a!)
  expect(a!.currentTime).toBe(2)
  fireEvent.click(screen.getByRole('button', { name: '재생' }))
  act(() => frames.get(a!)!(0, { mediaTime: 2.5 } as VideoFrameCallbackMetadata))
  expect(scrubber()).toHaveAttribute('aria-valuetext', '0.500 / 19.800 s')
  expect(a!.currentTime).toBe(2)
  fireEvent.click(screen.getByRole('button', { name: '일시 정지' }))
  expect(a!.currentTime).toBe(2.5)
})

it('keeps the footage when play() is interrupted, and pauses when the browser refuses it', async () => {
  const play = vi
    .spyOn(HTMLMediaElement.prototype, 'play')
    .mockRejectedValue(new DOMException('interrupted', 'AbortError'))
  const view = mount()
  await syncedPlayers(view.container, 2, (player) => expect(player).toHaveAttribute('src'))
  fireEvent.click(screen.getByRole('button', { name: '재생' }))
  await waitFor(() => expect(play).toHaveBeenCalled())
  await act(async () => {})
  expect(screen.queryByText(/원본을 재생하지 못했어요/)).not.toBeInTheDocument()
  expect(view.container.querySelectorAll('video')).toHaveLength(2)
  expect(screen.getByRole('button', { name: '일시 정지' })).toBeInTheDocument()
  play.mockRejectedValue(new DOMException('needs a gesture', 'NotAllowedError'))
  fireEvent.click(screen.getByRole('button', { name: '일시 정지' }))
  fireEvent.click(screen.getByRole('button', { name: '재생' }))
  expect(screen.getByRole('button', { name: '일시 정지' })).toBeInTheDocument()
  await screen.findByRole('button', { name: '재생' })
  expect(screen.queryByText(/원본을 재생하지 못했어요/)).not.toBeInTheDocument()
  expect(view.container.querySelectorAll('video')).toHaveLength(2)
})

it('plays from a press on the frame, and refreshes footage and captions on demand', async () => {
  const view = mount()
  await syncedPlayers(view.container, 2, (player) => expect(player).toHaveAttribute('src'))
  await screen.findByText('현재 편집 내용의 미리보기예요.')
  const frame = screen.getByRole('button', { name: '재생' })
  fireEvent.click(frame)
  expect(frame).toHaveAccessibleName('일시 정지')
  fireEvent.click(screen.getByRole('button', { name: '미리보기 새로고침' }))
  expect(frame).toHaveAccessibleName('재생')
  await waitFor(() => expect(view.access).toHaveBeenCalledWith('a', true))
  expect(view.access).toHaveBeenCalledWith('b', true)
  // The caption glyphs are the feature's to fetch again; the frame only asks for it.
  expect(view.preview.onRetry).toHaveBeenCalledTimes(1)
})

it('says the preparation failed and offers the way back, without knowing what failed', async () => {
  const preview = overlay({
    ready: false,
    failure: { reason: 'CLIP_PREVIEW_TOO_LARGE', params: {} },
  })
  mount('vertical', undefined, { preview })
  expect(
    await screen.findByText(
      '편집 내용의 미리보기를 준비하지 못했어요. 내용과 표시 시간을 확인해 주세요.',
    ),
  ).toBeVisible()
  fireEvent.click(screen.getByRole('button', { name: '미리보기 다시 준비' }))
  expect(preview.onRetry).toHaveBeenCalledTimes(1)
})
