import { afterEach, expect, it, vi } from 'vitest'
import { encodePng } from './resize'
import { nextQuarterTurn, quarterTurn } from './rotation'

afterEach(() => vi.unstubAllGlobals())

// POST-107: only the four quarter turns exist, and a 회전 press steps a quarter clockwise.
it('reads a turn as one of the four quarter turns and steps it clockwise', () => {
  expect([0, 90, 180, 270, 45, -90, 360, undefined].map(quarterTurn)).toEqual([
    0, 90, 180, 270, 0, 0, 0, 0,
  ])
  expect([undefined, 0, 90, 180, 270].map(nextQuarterTurn)).toEqual([90, 90, 180, 270, 0])
})

// EXPORT-15: the copy is encoded turned — a quarter turn swaps the canvas sides and draws the
// bitmap about the centre; no turn draws it as before.
it('encodes a turned photo on a canvas of the turned shape', async () => {
  const canvases: Array<{ width: number; height: number; calls: string[] }> = []
  vi.stubGlobal(
    'OffscreenCanvas',
    class {
      calls: string[] = []
      constructor(
        readonly width: number,
        readonly height: number,
      ) {
        canvases.push(this)
      }
      getContext() {
        return {
          translate: (x: number, y: number) => this.calls.push(`translate ${x} ${y}`),
          rotate: (angle: number) =>
            this.calls.push(`rotate ${Math.round((angle * 180) / Math.PI)}`),
          drawImage: (_: unknown, x: number, y: number) => this.calls.push(`draw ${x} ${y}`),
        }
      }
      convertToBlob({ type }: { type: string }) {
        return Promise.resolve(new Blob([new Uint8Array([1])], { type }))
      }
    },
  )
  const bitmap = { width: 400, height: 300 } as ImageBitmap
  await encodePng(bitmap)
  await encodePng(bitmap, 90)
  await encodePng(bitmap, 180)
  expect(canvases.map(({ width, height }) => [width, height])).toEqual([
    [400, 300],
    [300, 400],
    [400, 300],
  ])
  expect(canvases[0]!.calls).toEqual(['draw 0 0'])
  expect(canvases[1]!.calls).toEqual(['translate 150 200', 'rotate 90', 'draw -200 -150'])
})
