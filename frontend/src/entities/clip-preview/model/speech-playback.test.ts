import { describe, expect, it, vi } from 'vitest'
import type { ClipSpeechRef } from '@/entities/clip-plan/@x/clip-preview'
import {
  SpeechDecodeCache,
  SpeechPreviewTransport,
  canonicalSpeechBuffer,
  type ScheduledSpeech,
} from './speech-playback'

function speech(hash: string, start: number, duration: number): ScheduledSpeech {
  return {
    segmentId: hash,
    start,
    duration,
    speech: {
      assetId: hash,
      audioHash: hash,
      samples: duration * 44100,
      sampleRate: 44100,
      channels: 2,
    } as ClipSpeechRef,
  }
}
function harness(segments = [speech('a', 1, 4), speech('b', 8, 2)]) {
  const nodes: {
    start: ReturnType<typeof vi.fn>
    stop: ReturnType<typeof vi.fn>
    disconnect: ReturnType<typeof vi.fn>
    playbackRate: { value: number }
  }[] = []
  const gain = { gain: { setValueAtTime: vi.fn() }, connect: vi.fn(), disconnect: vi.fn() }
  const context = {
    state: 'running',
    currentTime: 100,
    sampleRate: 48000,
    destination: {},
    resume: vi.fn(async () => {}),
    close: vi.fn(async () => {}),
    decodeAudioData: vi.fn(async (bytes: ArrayBuffer) => {
      const duration = new Uint8Array(bytes)[0]!
      return { duration, length: duration * 48000, numberOfChannels: 2 } as AudioBuffer
    }),
    createGain: vi.fn(() => gain),
    createBufferSource: vi.fn(() => {
      const node = {
        start: vi.fn(),
        stop: vi.fn(),
        disconnect: vi.fn(),
        connect: vi.fn(),
        playbackRate: { value: 0 },
        buffer: undefined,
      }
      nodes.push(node)
      return node
    }),
  }
  const load = vi.fn(
    async (ref: ClipSpeechRef) => new Uint8Array([ref.samples / ref.sampleRate]).buffer,
  )
  const cache = new SpeechDecodeCache()
  const player = new SpeechPreviewTransport(
    segments,
    20,
    0.7,
    load,
    cache,
    () => context as unknown as AudioContext,
  )
  return { player, context, gain, nodes, load, cache }
}

describe('monotonic narration transport', () => {
  it('seeks into natural-rate speech, schedules across cuts/gaps and reuses immutable PCM', async () => {
    const h = harness()
    await h.player.play(2500)
    expect(h.nodes[0]!.start).toHaveBeenCalledWith(100, 1.5, 2.5)
    expect(h.nodes[1]!.start).toHaveBeenCalledWith(105.5, 0, 2)
    expect(h.nodes.map((n) => n.playbackRate.value)).toEqual([1, 1])
    h.context.currentTime = 103
    expect(h.player.timeMs).toBe(5500)
    h.player.pause()
    h.context.currentTime = 110
    expect(h.player.timeMs).toBe(5500)
    expect(h.nodes[0]!.stop).toHaveBeenCalledOnce()
    await h.player.play(5500)
    expect(h.load).toHaveBeenCalledTimes(2)
    expect(h.nodes).toHaveLength(3)
    expect(h.nodes[2]!.start).toHaveBeenCalledWith(112.5, 0, 2)
    h.player.dispose()
    expect(h.nodes[2]!.stop).toHaveBeenCalledOnce()
    expect(h.context.close).toHaveBeenCalledOnce()
  })
  it('mutes intentionally and freezes the output clock on visibility/context suspension', async () => {
    const h = harness()
    h.player.setMuted(true)
    await h.player.play(0)
    expect(h.gain.gain.setValueAtTime).toHaveBeenCalledWith(0, 100)
    h.player.setMuted(false)
    expect(h.gain.gain.setValueAtTime).toHaveBeenLastCalledWith(0.7, 100)
    h.context.currentTime = 101
    h.context.state = 'suspended'
    expect(h.player.running).toBe(false)
    h.player.pause()
    h.context.currentTime = 120
    expect(h.player.timeMs).toBe(1000)
  })
  it('cancels late decode before creating playback nodes', async () => {
    const h = harness()
    let release!: (bytes: ArrayBuffer) => void
    h.load.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          release = resolve
        }),
    )
    const playing = h.player.play(0)
    await vi.waitFor(() => expect(h.load).toHaveBeenCalled())
    h.player.pause()
    release(new Uint8Array([4]).buffer)
    expect(await playing).toBe(false)
    expect(h.nodes).toEqual([])
  })
  it('names invalid decoding, authorization failure and gesture refusal without playing partial audio', async () => {
    const h = harness()
    h.context.decodeAudioData.mockRejectedValueOnce(new Error('codec'))
    await expect(h.player.play(0)).rejects.toMatchObject({ reason: 'decode', segmentId: 'a' })
    expect(h.nodes).toEqual([])
    h.load.mockRejectedValueOnce(new Error('revoked'))
    await expect(h.player.play(0)).rejects.toMatchObject({ reason: 'unavailable', segmentId: 'a' })
    h.context.resume.mockRejectedValueOnce(new Error('gesture'))
    await expect(h.player.play(0)).rejects.toMatchObject({ reason: 'gesture' })
  })
  it('refuses decoded memory before fetching and keeps zero gain scheduled', async () => {
    const h = harness([speech('huge', 0, 120)])
    await expect(h.player.play(0)).rejects.toMatchObject({ reason: 'memory' })
    expect(h.load).not.toHaveBeenCalled()
    const normal = harness()
    const muted = new SpeechPreviewTransport(
      [speech('a', 0, 4)],
      20,
      0,
      normal.load,
      normal.cache,
      () => normal.context as unknown as AudioContext,
    )
    await muted.play(0)
    expect(normal.nodes).toHaveLength(1)
    expect(normal.gain.gain.setValueAtTime).toHaveBeenCalledWith(0, 100)
    muted.dispose()
  })
})

it('pads gapless codec silence to measured samples without stretching or trimming speech', () => {
  const decoded = {
    duration: 2,
    length: 96000,
    numberOfChannels: 2,
    getChannelData: () => new Float32Array(96000).fill(0.1),
  } as unknown as AudioBuffer
  const channels: Float32Array[] = []
  const context = {
    sampleRate: 48000,
    createBuffer: (_channels: number, length: number) => {
      channels.push(new Float32Array(length), new Float32Array(length))
      return {
        length,
        copyToChannel: (data: Float32Array, channel: number) => channels[channel]!.set(data),
      } as AudioBuffer
    },
  }
  const ref = { samples: 91008, sampleRate: 44100, channels: 2 } as ClipSpeechRef
  const result = canonicalSpeechBuffer(context, ref, decoded)
  expect(result.length).toBe(Math.ceil((91008 * 48000) / 44100))
  expect(channels[0]![95999]).toBeCloseTo(0.1)
  expect(channels[0]![96000]).toBe(0)
  expect(() => canonicalSpeechBuffer(context, { ...ref, samples: 96000 }, decoded)).toThrow(
    'decode',
  )
  expect(() => canonicalSpeechBuffer(context, { ...ref, samples: 44100 }, decoded)).toThrow(
    'decode',
  )
})
