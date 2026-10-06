import { describe, expect, it, vi } from 'vitest'
import { CLIP_DESIGN, clipInkMetrics, clipTextWidth } from '@/entities/clip-design/@x/clip-preview'
import { inkLayoutCaption } from './ink-layout'
import { inkRuns, type InkRole } from './ink-typography'
import { inkStaticCaption } from './ink-static'
import type { BrowserInkRasterizer } from './ink-raster'

/** Font-independent layout port: the actual WASM/native shaping is browser-qualified. */
const layoutRasterizer = (): BrowserInkRasterizer => ({
  measure: vi.fn(async (text: string, role: InkRole, caption: boolean) => {
    const runs = inkRuns(text, role, caption)
    let ascent = 0,
      descent = 0,
      width = 0
    for (const run of runs) {
      const m = clipInkMetrics(run.face, role.weight)
      ascent = Math.max(ascent, (100 * m.ascent) / m.units)
      descent = Math.max(descent, (-100 * m.descent) / m.units)
      const weight = run.face === 'wantedsans' ? 700 : role.weight
      width += clipTextWidth(run.face, weight, role.tracking, 100, run.text)
    }
    return { x: 0, y: -ascent, width, height: ascent + descent }
  }),
  render: vi.fn(),
  destroy: vi.fn(),
})
const input = {
  text: '정확한 글자',
  style: 'bold',
  ratio: 'vertical' as const,
  position: 'bottom',
  align: 'center',
  keyword: '',
  pace: 'steady',
}
describe('native caption layout', () => {
  it('prefers the same clean word break, respects authored newlines and never shrinks an authored size', async () => {
    const raster = layoutRasterizer()
    const wrapped = await inkLayoutCaption(
      { ...input, text: '한글 한글 한글 한글 한글 한글' },
      raster,
    )
    expect(wrapped.lines).toHaveLength(2)
    expect(wrapped.lines[0]!.text.endsWith(' ') || wrapped.lines[1]!.text.startsWith(' ')).toBe(
      true,
    )
    const authored = await inkLayoutCaption(
      { ...input, text: '첫째\n둘째', ownerSizePx: 64 },
      raster,
    )
    expect(authored.lines.map((line) => line.text)).toEqual(['첫째', '둘째'])
    expect(authored.role.size).toBe(64)
    await expect(
      inkLayoutCaption({ ...input, text: '한'.repeat(30), ownerSizePx: 72 }, raster),
    ).rejects.toThrow('CLIP_INK_COPY_LIMIT')
  })
  it('clamps owner geometry and uses identical normalized ink for movement-only changes', async () => {
    const raster = layoutRasterizer()
    const a = await inkLayoutCaption({ ...input, ownerPosition: { x: -50, y: 20 } }, raster)
    expect(a.region.x).toBe(64)
    expect(a.region.y).toBe(40)
    const b = await inkLayoutCaption({ ...input, ownerPosition: { x: 160, y: 600 } }, raster)
    expect(inkStaticCaption('vertical', a).key).toBe(inkStaticCaption('vertical', b).key)
    expect(inkStaticCaption('vertical', a).key).not.toBe(inkStaticCaption('square', a).key)
    expect(inkStaticCaption('vertical', a, { accent: CLIP_DESIGN.accent.coral }).key).toBe(
      inkStaticCaption('vertical', a).key,
    )
    const keyword = await inkLayoutCaption({ ...input, keyword: '글자' }, raster)
    expect(
      inkStaticCaption('vertical', keyword, { accent: CLIP_DESIGN.accent.coral }).key,
    ).not.toBe(inkStaticCaption('vertical', keyword, { accentWhite: true }).key)
  })
  it('refuses unresolved automatic anchors rather than silently choosing a default', async () => {
    await expect(
      inkLayoutCaption({ ...input, position: 'auto' }, layoutRasterizer()),
    ).rejects.toThrow('CLIP_INK_UNRESOLVED_GEOMETRY')
  })
})
