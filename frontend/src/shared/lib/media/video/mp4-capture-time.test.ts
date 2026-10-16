import { Blob as NodeBlob } from 'node:buffer'
import { describe, expect, it, vi } from 'vitest'
import { readMp4CaptureTime } from './mp4-capture-time'

function box(tag: string, body: Uint8Array, extended = false) {
  const header = extended ? 16 : 8
  const out = new Uint8Array(header + body.length)
  const view = new DataView(out.buffer)
  view.setUint32(0, extended ? 1 : out.length)
  out.set(new TextEncoder().encode(tag), 4)
  if (extended) view.setBigUint64(8, BigInt(out.length))
  out.set(body, header)
  return out
}
function movie(iso: string | bigint, version = 0, extended = false) {
  const seconds = typeof iso === 'bigint' ? iso : BigInt(Date.parse(iso) / 1000) + 2_082_844_800n
  const data = new Uint8Array(version === 1 ? 112 : 100)
  data[0] = version
  const view = new DataView(data.buffer)
  if (version === 1) view.setBigUint64(4, seconds)
  else view.setUint32(4, Number(seconds))
  return box('moov', box('mvhd', data, extended), extended)
}
const blob = (bytes: Uint8Array) => new NodeBlob([bytes]) as unknown as Blob

describe('MP4/MOV capture time', () => {
  it('reads creation time', async () => {
    expect(await readMp4CaptureTime(blob(movie('2026-10-15T03:34:56Z')))).toBe(
      '2026-10-15T03:34:56.000Z',
    )
  })
  it('reads64-bit creation time and extended-size boxes past2040', async () => {
    expect(await readMp4CaptureTime(blob(movie('2050-01-02T03:04:05Z', 1, true)))).toBe(
      '2050-01-02T03:04:05.000Z',
    )
  })
  it('treats1904 epoch zero as absent', async () => {
    expect(await readMp4CaptureTime(blob(movie(0n)))).toBeUndefined()
  })
  it('ignores dates before2000 and unsafe64-bit values', async () => {
    expect(await readMp4CaptureTime(blob(movie('1999-12-31T23:59:59Z')))).toBeUndefined()
    expect(await readMp4CaptureTime(blob(movie(2n ** 64n - 1n, 1)))).toBeUndefined()
  })
  it('ignores missing, invalid-version and truncated headers', async () => {
    expect(
      await readMp4CaptureTime(blob(box('moov', box('free', new Uint8Array(16))))),
    ).toBeUndefined()
    expect(await readMp4CaptureTime(blob(movie('2026-10-15T03:34:56Z', 2)))).toBeUndefined()
    expect(
      await readMp4CaptureTime(blob(movie('2026-10-15T03:34:56Z').slice(0, 20))),
    ).toBeUndefined()
  })
  it('skips mdat without reading its bytes', async () => {
    const file = new NodeBlob([
      box('ftyp', new Uint8Array(8)),
      box('mdat', new Uint8Array(8 * 1024 * 1024)),
      movie('2026-10-15T03:34:56Z'),
    ]) as unknown as Blob
    const slices = vi.spyOn(file, 'slice')
    expect(await readMp4CaptureTime(file)).toBe('2026-10-15T03:34:56.000Z')
    const bytes = slices.mock.calls.reduce(
      (sum, [start, end]) => sum + Number(end) - Number(start),
      0,
    )
    expect(bytes).toBeLessThanOrEqual(128)
    expect(slices.mock.calls.every(([start, end]) => Number(end) - Number(start) <= 32)).toBe(true)
  })
  it('bounds header reads even for a pathological box count', async () => {
    const file = new NodeBlob([
      ...Array.from({ length: 10000 }, () => box('free', new Uint8Array())),
      movie('2026-10-15T03:34:56Z'),
    ]) as unknown as Blob
    const slices = vi.spyOn(file, 'slice')
    expect(await readMp4CaptureTime(file)).toBeUndefined()
    expect(
      slices.mock.calls.reduce((sum, [start, end]) => sum + Number(end) - Number(start), 0),
    ).toBeLessThanOrEqual(256 * 1024)
  })
  it('aborts without continuing bounded reads', async () => {
    const controller = new AbortController()
    controller.abort()
    await expect(
      readMp4CaptureTime(blob(movie('2026-10-15T03:34:56Z')), controller.signal),
    ).rejects.toMatchObject({ name: 'AbortError' })
  })
})
