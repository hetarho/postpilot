import { afterEach, describe, expect, it, vi } from 'vitest'
import { formatDuration, readVideoMetadata, VideoUnreadableError } from './metadata'

/** A stand-in for the platform's own demuxer. The real element is asynchronous and driven by
 *  bytes; this one is driven by the test, which is the only way to pin what happens on each of
 *  the three outcomes without shipping a fixture clip. */
function fakeVideoElement() {
  const listeners = new Map<string, () => void>()
  const element = {
    preload: '',
    muted: false,
    src: '',
    duration: 0,
    videoWidth: 0,
    videoHeight: 0,
    addEventListener: (type: string, handler: () => void) => listeners.set(type, handler),
    removeAttribute: vi.fn(),
    load: vi.fn(),
  }
  return { element, fire: (type: string) => listeners.get(type)?.() }
}

function stub(video: ReturnType<typeof fakeVideoElement>) {
  const created = vi.spyOn(document, 'createElement').mockReturnValue(video.element as never)
  const objectUrl = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:clip')
  const revoked = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  return { created, objectUrl, revoked }
}

const file = () => new Blob(['bytes'], { type: 'video/mp4' })

afterEach(() => {
  vi.restoreAllMocks()
  vi.useRealTimers()
})

describe('readVideoMetadata', () => {
  // VIDEO-4: nothing decodes a frame. The element only ever loads metadata, and the object URL
  // is revoked either way — a leaked one pins the whole 200 MB file in memory.
  it('reads the duration and dimensions without decoding, then revokes the url', async () => {
    const video = fakeVideoElement()
    const spies = stub(video)

    const reading = readVideoMetadata(file())
    expect(video.element.preload).toBe('metadata')
    expect(video.element.muted).toBe(true)
    expect(video.element.src).toBe('blob:clip')

    video.element.duration = 12.4
    video.element.videoWidth = 1920
    video.element.videoHeight = 1080
    video.fire('loadedmetadata')

    await expect(reading).resolves.toEqual({ durationMs: 12400, width: 1920, height: 1080 })
    expect(spies.revoked).toHaveBeenCalledWith('blob:clip')
    // The source is cleared and reloaded, which is what actually stops a read still in flight.
    expect(video.element.removeAttribute).toHaveBeenCalledWith('src')
    expect(video.element.load).toHaveBeenCalled()
  })

  it('rejects a container the platform cannot parse', async () => {
    const video = fakeVideoElement()
    const spies = stub(video)

    const reading = readVideoMetadata(file())
    video.fire('error')

    await expect(reading).rejects.toBeInstanceOf(VideoUnreadableError)
    expect(spies.revoked).toHaveBeenCalledWith('blob:clip')
  })

  // A stream with no known duration reports Infinity, and one with no video track reports 0×0.
  // Neither can describe a clip the post could show, so both are unreadable here rather than a
  // confirm the server would refuse.
  it.each([
    { duration: Infinity, width: 1920, height: 1080 },
    { duration: 0, width: 1920, height: 1080 },
    { duration: 5, width: 0, height: 0 },
  ])('rejects metadata that cannot describe a clip (%o)', async (metadata) => {
    const video = fakeVideoElement()
    stub(video)

    const reading = readVideoMetadata(file())
    video.element.duration = metadata.duration
    video.element.videoWidth = metadata.width
    video.element.videoHeight = metadata.height
    video.fire('loadedmetadata')

    await expect(reading).rejects.toBeInstanceOf(VideoUnreadableError)
  })

  // A container the platform silently refuses to parse fires nothing at all; without the bound
  // the card would sit at 읽는 중 forever.
  it('rejects after a bounded wait when nothing fires', async () => {
    vi.useFakeTimers()
    const video = fakeVideoElement()
    const spies = stub(video)

    const reading = readVideoMetadata(file())
    const settled = expect(reading).rejects.toBeInstanceOf(VideoUnreadableError)
    await vi.advanceTimersByTimeAsync(15_000)
    await settled
    expect(spies.revoked).toHaveBeenCalledWith('blob:clip')
  })
})

describe('formatDuration', () => {
  it('pads the seconds so the badge keeps one width', () => {
    expect(formatDuration(0)).toBe('0:00')
    expect(formatDuration(9_000)).toBe('0:09')
    expect(formatDuration(65_400)).toBe('1:05')
    expect(formatDuration(600_000)).toBe('10:00')
  })
})
