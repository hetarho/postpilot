import { describe, expect, it, vi } from 'vitest'
import type { ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import { BrowserCompositionPlayback } from './composition-playback'

const plan: ClipEditPlan = {
  durationMs: 15000,
  cuts: [
    {
      id: 'cut',
      sourceId: 'source',
      fingerprint: 'a'.repeat(64),
      startMs: 0,
      endMs: 15000,
      transitionMs: 0,
      volumePermille: 1000,
      playbackRatePermille: 1000,
      copies: [],
    },
  ],
  sourceAudio: [{ sourceId: 'source', fingerprint: 'a'.repeat(64), retainOriginalAudio: false }],
}
function clock(resume = vi.fn(async () => {})) {
  const context = {
    currentTime: 0,
    sampleRate: 48000,
    state: 'running',
    resume,
    close: vi.fn(async () => {}),
    destination: {},
    createGain: vi.fn(() => ({
      gain: { value: 1, setValueAtTime: vi.fn(), linearRampToValueAtTime: vi.fn() },
      connect: vi.fn(),
      disconnect: vi.fn(),
    })),
  }
  return context
}
describe('one preview output clock', () => {
  it('advances silent/source-off footage on the same gesture-resumed AudioContext and releases on pause', async () => {
    const context = clock(),
      access = vi.fn(),
      load = vi.fn()
    const transport = new BrowserCompositionPlayback(
      plan,
      access,
      load,
      () => context as unknown as AudioContext,
    )
    const playing = transport.play(1000)
    expect(context.resume).toHaveBeenCalledOnce()
    expect(await playing).toBe(true)
    context.currentTime = 2
    expect(transport.timeMs).toBe(3000)
    expect(access).not.toHaveBeenCalled()
    expect(load).not.toHaveBeenCalled()
    transport.pause()
    context.currentTime = 4
    expect(transport.timeMs).toBe(3000)
    expect(transport.measurements().liveNodes).toBe(0)
    transport.dispose()
    expect(context.close).toHaveBeenCalledOnce()
  })
  it('cannot schedule old work after pause while gesture resumption is still pending', async () => {
    let resumed!: () => void
    const context = clock(
      vi.fn(
        () =>
          new Promise<void>((resolve) => {
            resumed = resolve
          }),
      ),
    )
    const transport = new BrowserCompositionPlayback(
      plan,
      vi.fn(),
      vi.fn(),
      () => context as unknown as AudioContext,
    )
    const pending = transport.play(500)
    transport.pause()
    resumed()
    expect(await pending).toBe(false)
    expect(context.createGain).not.toHaveBeenCalled()
    expect(transport.running).toBe(false)
    transport.dispose()
  })
})
