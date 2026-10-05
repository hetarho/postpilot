import { Input } from 'mediabunny'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { mp4AudioDecodedBytes } from './mp4-audio'

const fake = { tracks: vi.fn(), dispose: vi.fn() }
beforeEach(() => {
  vi.spyOn(Input.prototype, 'getAudioTracks').mockImplementation(fake.tracks)
  vi.spyOn(Input.prototype, 'dispose').mockImplementation(fake.dispose)
})
afterEach(() => {
  vi.restoreAllMocks()
  vi.clearAllMocks()
})
it('reserves the entire retained original rather than just its selected cut', async () => {
  fake.tracks.mockResolvedValue([{ numberOfChannels: 2, computeDuration: async () => 1800 }])
  const bytes = await mp4AudioDecodedBytes(new Blob(), 48000, new AbortController().signal)
  expect(bytes).toBeGreaterThan(256 * 1024 * 1024)
  expect(fake.dispose).toHaveBeenCalledOnce()
})
it('includes multichannel decode and stereo conversion, and allocates nothing for video-only input', async () => {
  fake.tracks.mockResolvedValue([{ numberOfChannels: 6, computeDuration: async () => 10 }])
  expect(await mp4AudioDecodedBytes(new Blob(), 48000, new AbortController().signal)).toBe(
    Math.ceil(10.1 * 48000) * 8 * 4,
  )
  fake.tracks.mockResolvedValue([])
  expect(await mp4AudioDecodedBytes(new Blob(), 48000, new AbortController().signal)).toBe(0)
})
it('refuses unknown metadata and disposes the demuxer', async () => {
  fake.tracks.mockResolvedValue([{ numberOfChannels: 2, computeDuration: async () => Infinity }])
  await expect(
    mp4AudioDecodedBytes(new Blob(), 48000, new AbortController().signal),
  ).rejects.toThrow('Invalid original audio metadata')
  expect(fake.dispose).toHaveBeenCalledOnce()
})
