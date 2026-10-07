import { describe, expect, it, vi } from 'vitest'
import { bindNativeSourceColor, nativeSourceColorSpace } from './source-color'

describe('native original color conversion', () => {
  it('uses the evidenced native601 default for absent tags and preserves tagged709 matrix/range', () => {
    expect(nativeSourceColorSpace({})).toEqual({
      matrix: 'smpte170m',
      primaries: 'bt709',
      transfer: 'bt709',
      fullRange: false,
    })
    expect(
      nativeSourceColorSpace({
        matrix: 'bt709',
        primaries: 'bt709',
        transfer: 'bt709',
        fullRange: true,
      }),
    ).toEqual({ matrix: 'bt709', primaries: 'bt709', transfer: 'bt709', fullRange: true })
  })
  it.each([
    { matrix: 'bt2020-ncl' },
    { transfer: 'pq' },
    { transfer: 'hlg' },
    { primaries: 'smpte432' },
  ] as unknown as VideoColorSpaceInit[])(
    'refuses an unsupported actual color condition %j without relabeling it',
    (color) => {
      expect(() => nativeSourceColorSpace(color)).toThrow('CLIP_SOURCE_COLOR_UNSUPPORTED')
    },
  )
  it('binds only its owned track config, preserving codec bytes and independent source instances', async () => {
    const config = {
      codec: 'avc1.640028',
      description: new Uint8Array([1, 2, 3]),
      colorSpace: { matrix: 'bt709' },
    } satisfies VideoDecoderConfig
    const track = { getDecoderConfig: vi.fn(async () => config) }
    const other = { getDecoderConfig: vi.fn(async () => config) }
    bindNativeSourceColor(track, nativeSourceColorSpace({}))
    const bound = await track.getDecoderConfig()
    expect(bound?.colorSpace?.matrix).toBe('smpte170m')
    expect(bound?.description).toBe(config.description)
    expect((await other.getDecoderConfig()).colorSpace.matrix).toBe('bt709')
    expect(config.colorSpace.matrix).toBe('bt709')
  })
})
