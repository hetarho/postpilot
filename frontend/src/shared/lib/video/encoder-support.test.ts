import { afterEach, expect, it, vi } from 'vitest'
import { probeEncoderSupport } from './encoder-support'

afterEach(() => vi.unstubAllGlobals())

const video = { codec: 'avc1.640028', width: 1080, height: 1920, framerate: 30 }
const audio = { codec: 'mp4a.40.2', sampleRate: 48000, numberOfChannels: 2 }

it('asks the actual static APIs without constructing an encoder or fetching any original', async () => {
  const fetch = vi.fn()
  vi.stubGlobal('fetch', fetch)
  const Video = Object.assign(vi.fn(), {
    isConfigSupported: vi.fn(async () => ({ supported: true })),
  })
  const Audio = Object.assign(vi.fn(), {
    isConfigSupported: vi.fn(async () => ({ supported: true })),
  })
  vi.stubGlobal('VideoEncoder', Video)
  vi.stubGlobal('AudioEncoder', Audio)
  expect(await probeEncoderSupport(video, audio)).toMatchObject({ video: true, audio: true })
  expect(Video.isConfigSupported).toHaveBeenCalledExactlyOnceWith(video)
  expect(Audio.isConfigSupported).toHaveBeenCalledExactlyOnceWith(audio)
  expect(Video).not.toHaveBeenCalled()
  expect(Audio).not.toHaveBeenCalled()
  expect(fetch).not.toHaveBeenCalled()
})

it('skips the audio API for a video-only plan', async () => {
  const Audio = { isConfigSupported: vi.fn(async () => ({ supported: false })) }
  expect(
    await probeEncoderSupport(video, undefined, {
      video: { isConfigSupported: async () => ({ supported: true }) },
      audio: Audio,
      deviceMemoryGB: 4,
    }),
  ).toEqual({ video: true, audio: true, deviceMemoryGB: 4 })
  expect(Audio.isConfigSupported).not.toHaveBeenCalled()
})

it('treats missing APIs and thrown capability queries as unsupported', async () => {
  expect(await probeEncoderSupport(video, audio, {})).toMatchObject({ video: false, audio: false })
  expect(
    await probeEncoderSupport(video, audio, {
      video: {
        isConfigSupported: () => {
          throw new TypeError('unsupported config')
        },
      },
      audio: {
        isConfigSupported: async () => {
          throw new Error('unavailable')
        },
      },
    }),
  ).toMatchObject({ video: false, audio: false })
})
