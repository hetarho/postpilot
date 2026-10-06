import { describe, expect, it } from 'vitest'
import { clipInkCoverage, clipInkFont } from '@/entities/clip-design/@x/clip-preview'
import { inkResolveCaption, inkRuns, inkValidateText } from './ink-typography'

describe('exact bundled glyph authority', () => {
  it('rejects cmap entries without outlines and substitutes rare syllables within the same caption style', () => {
    expect(clipInkCoverage('paperlogy', 800, '갂')).toBe(false)
    expect(clipInkCoverage('wantedsans', 800, '갂')).toBe(true)
    const result = inkResolveCaption('bold', '한 갂 글')
    expect(result.style.id).toBe('bold')
    expect(result.notice).toBe('')
    expect(
      inkRuns('한 갂 글', { face: 'paperlogy', weight: 800, tracking: -0.02, size: 72 }, true),
    ).toContainEqual({ text: '갂', face: 'wantedsans', substituted: true })
  })
  it('uses only the defined whole-caption fallback and refuses unsupported information/region glyphs', () => {
    expect(inkResolveCaption('pop', '2㎏').style.id).toBe('bold')
    expect(inkResolveCaption('pop', '2㎏').notice).toBe('composition_caption_glyph')
    expect(() => inkResolveCaption('bold', 'α А')).toThrow('CLIP_INK_UNSUPPORTED_GLYPH')
    expect(() =>
      inkRuns('갂', { face: 'paperlogy', weight: 800, tracking: 0, size: 72 }, false),
    ).toThrow('CLIP_INK_UNSUPPORTED_GLYPH')
    expect(() => clipInkFont('system-font', 800)).toThrow('CLIP_INK_UNSUPPORTED_FONT')
    expect(() => clipInkFont('wantedsans', 900)).toThrow('CLIP_INK_UNSUPPORTED_FONT')
  })
  it('keeps spaces and rejects invalid controls without selecting a replacement font', () => {
    expect(clipInkCoverage('jua', 400, ' ')).toBe(true)
    expect(() => inkValidateText('exact\u0000copy')).toThrow('CLIP_INK_INVALID_TEXT')
    expect(() => inkValidateText('one\ntwo', true)).not.toThrow()
  })
})
