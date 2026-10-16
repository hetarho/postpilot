import { Blob as NodeBlob } from 'node:buffer'
import { describe, expect, it, vi } from 'vitest'
import { readJpegCaptureTime } from './jpeg-capture-time'

function jpeg({
  little = true,
  date = '2026:10:15 12:34:56',
  offset = '+09:00',
  dateTag = 0x9003,
} = {}) {
  const data = new Uint8Array(83)
  const view = new DataView(data.buffer)
  view.setUint16(0, little ? 0x4949 : 0x4d4d)
  view.setUint16(2, 42, little)
  view.setUint32(4, 8, little)
  view.setUint16(8, 1, little)
  view.setUint16(10, 0x8769, little)
  view.setUint16(12, 4, little)
  view.setUint32(14, 1, little)
  view.setUint32(18, 26, little)
  view.setUint16(26, 2, little)
  for (const [at, tag, text, value] of [
    [28, dateTag, date, 56],
    [40, 0x9011, offset, 76],
  ] as const) {
    view.setUint16(at, tag, little)
    view.setUint16(at + 2, 2, little)
    view.setUint32(at + 4, text.length + 1, little)
    view.setUint32(at + 8, value, little)
    data.set(new TextEncoder().encode(text), value)
  }
  const out = new Uint8Array(14 + data.length)
  out.set([0xff, 0xd8, 0xff, 0xe1])
  new DataView(out.buffer).setUint16(4, 8 + data.length)
  out.set([0x45, 0x78, 0x69, 0x66, 0, 0], 6)
  out.set(data, 12)
  out.set([0xff, 0xd9], out.length - 2)
  return out
}
const blob = (bytes: Uint8Array) => new NodeBlob([bytes]) as unknown as Blob

describe('JPEG capture time', () => {
  it.each([true, false])(
    'reads DateTimeOriginal with offset (little endian=%s)',
    async (little) => {
      expect(await readJpegCaptureTime(blob(jpeg({ little })))).toBe('2026-10-15T03:34:56.000Z')
    },
  )
  it('reads a negative offset without using the browser timezone', async () => {
    expect(await readJpegCaptureTime(blob(jpeg({ offset: '-03:30' })))).toBe(
      '2026-10-15T16:04:56.000Z',
    )
  })
  it('ignores a JPEG without EXIF', async () => {
    expect(
      await readJpegCaptureTime(blob(Uint8Array.of(0xff, 0xd8, 0xff, 0xdb, 0, 2, 0xff, 0xda))),
    ).toBeUndefined()
  })
  it('ignores non-JPEG', async () => {
    const bytes = jpeg()
    bytes[0] = 0x89
    expect(await readJpegCaptureTime(blob(bytes))).toBeUndefined()
  })
  it.each([
    { offset: '' },
    { offset: '+25:00' },
    { date: '2026:02:30 12:34:56' },
    { dateTag: 0x9004 },
  ])('omits unusable original instants: %j', async (options) => {
    expect(await readJpegCaptureTime(blob(jpeg(options)))).toBeUndefined()
  })
  it('ignores out-of-bounds TIFF pointers and truncated tags', async () => {
    const bytes = jpeg()
    new DataView(bytes.buffer).setUint32(12 + 28 + 8, 1_000_000, true)
    expect(await readJpegCaptureTime(blob(bytes))).toBeUndefined()
    expect(await readJpegCaptureTime(blob(jpeg().slice(0, 80)))).toBeUndefined()
  })
  it('reads at most the original first128KiB', async () => {
    const padding = new Uint8Array(65537)
    padding.set([0xff, 0xe2, 0xff, 0xff])
    const file = new NodeBlob([
      Uint8Array.of(0xff, 0xd8),
      padding,
      padding,
      jpeg().slice(2),
    ]) as unknown as Blob
    const slices = vi.spyOn(file, 'slice')
    expect(await readJpegCaptureTime(file)).toBeUndefined()
    expect(slices).toHaveBeenCalledExactlyOnceWith(0, 128 * 1024)
  })
})
