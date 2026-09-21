import { beforeEach, describe, expect, it, vi } from 'vitest'
import { CaptionSheets, type CaptionFramePage } from './caption-sheets'

const bitmaps: { source: unknown; crop?: number[]; close: ReturnType<typeof vi.fn> }[] = []
vi.stubGlobal('createImageBitmap', async (source: unknown, ...crop: number[]) => {
  const bitmap = { source, crop: crop.length ? crop : undefined, close: vi.fn() }
  bitmaps.push(bitmap)
  return bitmap as unknown as ImageBitmap
})

function page(offset: number, cells: number, next: number): CaptionFramePage {
  return {
    sheet: new Uint8Array([offset]),
    cellWidth: 900,
    cellHeight: 250,
    columns: 4,
    cells,
    x: 90,
    y: 640,
    firstFrame: 30,
    frameOffset: offset,
    nextOffset: next,
  }
}

describe('the sheets a browser render draws a sequence caption from', () => {
  beforeEach(() => {
    bitmaps.length = 0
  })
  it('fetches one run at a time and blits the cell for each output frame', async () => {
    const load = vi.fn(async (_id: string, offset: number) =>
      offset === 0 ? page(0, 4, 4) : page(4, 2, -1),
    )
    const sheets = new CaptionSheets(load)
    const signal = new AbortController().signal
    // Inside the first run: one fetch, one decode, a cell per frame.
    const first = await sheets.cell('caption', 30, signal)
    expect(first?.x).toBe(90)
    expect(first?.width).toBe(900)
    expect(load).toHaveBeenCalledTimes(1)
    const third = await sheets.cell('caption', 32, signal)
    expect(load).toHaveBeenCalledTimes(1)
    expect(third?.bitmap).not.toBe(first?.bitmap)
    // The run holds cells 0..3, so frame 34 is the next run — asked for by the
    // frame the walk reached, not from the caption's start.
    await sheets.cell('caption', 34, signal)
    expect(load).toHaveBeenCalledTimes(2)
    expect(load.mock.calls[1][1]).toBe(4)
    // A frame the run the server answered with does not carry draws nothing
    // rather than drawing the wrong cell.
    load.mockImplementation(async () => page(4, 2, -1))
    expect(await sheets.cell('caption', 60, signal)).toBeUndefined()
  })
  it('holds one decoded sheet per caption and releases it with the walk', async () => {
    const load = vi.fn(async (_id: string, offset: number) =>
      offset === 0 ? page(0, 4, 4) : page(4, 4, -1),
    )
    const sheets = new CaptionSheets(load)
    const signal = new AbortController().signal
    await sheets.cell('caption', 30, signal)
    const decoded = bitmaps.filter((b) => !b.crop)
    await sheets.cell('caption', 34, signal)
    expect(decoded[decoded.length - 1].close).toHaveBeenCalledOnce()
    sheets.dispose()
    expect(bitmaps.filter((b) => !b.crop).every((b) => b.close.mock.calls.length === 1)).toBe(true)
  })
  it('refuses a run the server could not draw', async () => {
    const sheets = new CaptionSheets(async () => page(0, 0, -1))
    await expect(sheets.cell('caption', 30, new AbortController().signal)).rejects.toThrow(
      'CLIP_CAPTION_FRAMES_UNAVAILABLE',
    )
  })
})
