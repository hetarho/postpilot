import { describe, expect, it } from 'vitest'
import { render } from '@testing-library/react'
import { readFileSync } from 'node:fs'
import { CLIP_DESIGN, CLIP_STYLES, clipCaption, CLIP_TYPE } from '@/shared/config'
import { CopyStylePreview } from './CopyStylePreview'

/** The attributes the preview draws, per style: not pixels, and not a copy of
 *  the numbers — every one is compared against the design system itself. */
describe('copy style preview', () => {
  it.each(CLIP_STYLES)('draws %s exactly as the design system defines it', (style) => {
    const { container } = render(<CopyStylePreview style={style} accent="coral" text="오늘" />)
    const svg = container.querySelector(`[data-copy-style="${style}"]`)!
    const text = svg.querySelector('text')!
    const rule = clipCaption()
    const role = CLIP_TYPE.title
    expect(text.getAttribute('font-size')).toBe(String(role.size))
    expect(text.getAttribute('font-weight')).toBe(String(role.weight))
    expect(text.getAttribute('letter-spacing')).toBe(String(role.tracking * role.size))
    // A plated style paints its plate at the token's own opacity and radius; an
    // unplated one paints the stroke and the drop shadow instead.
    const plate = svg.querySelector('rect[fill-opacity]')
    expect(plate).toBeNull()
    expect(text.getAttribute('stroke-width')).toBe(String(CLIP_DESIGN.spacing.stroke_text))
    expect(text.getAttribute('font-family')).toBe(CLIP_DESIGN.faces.paperlogy)
    expect(svg.querySelector('feDropShadow')).not.toBeNull()
    // 깔끔하게 carries the accent bar and 메모 the accent dot (CDS-23, CDS-24).
    expect(svg.querySelectorAll('circle').length).toBe(rule.dot ? 1 : 0)
  })
  it('highlights the keyword only for 형광펜 and colours it only for 크게 강조', () => {
    for (const style of CLIP_STYLES) {
      const { container } = render(
        <CopyStylePreview style={style} accent="amber" keyword="9900원" text="가격 9900원" />,
      )
      const svg = container.querySelector(`[data-copy-style="${style}"]`)!
      const highlighted = svg.querySelector('rect[fill-opacity="0.9"]') !== null
      expect(highlighted).toBe(clipCaption().highlight)
      expect(svg.querySelector('tspan') !== null).toBe(
        !clipCaption().highlight && clipCaption().stroke !== '',
      )
    }
  })
  it('paints no accent at all when the template has none', () => {
    for (const style of CLIP_STYLES) {
      const { container } = render(<CopyStylePreview style={style} text="오늘" />)
      const svg = container.querySelector(`[data-copy-style="${style}"]`)!
      expect(svg.querySelectorAll('circle').length).toBe(0)
      expect(svg.querySelector('rect[fill-opacity="0.9"]')).toBeNull()
    }
  })
})

/** The palette is where components read colour (ARCH-20), and the design file is
 *  what the renderer embeds. They have to be the same hexes. */
describe('clip palette', () => {
  it('carries the design system’s own hexes', () => {
    const css = readFileSync('src/app/styles/index.css', 'utf8')
    const declared = (name: string) =>
      css.match(new RegExp(`--palette-clip-${name}:\\s*(#[0-9a-fA-F]{3,8});`))?.[1]?.toLowerCase()
    for (const [accent, hex] of Object.entries(CLIP_DESIGN.accent)) {
      expect(declared(accent)).toBe(hex.toLowerCase())
    }
    for (const [name, token] of Object.entries({
      stroke: CLIP_DESIGN.color.stroke_dark,
      badge: CLIP_DESIGN.color.badge_ad,
    })) {
      expect(declared(name)).toBe(token.hex.toLowerCase())
    }
  })
})
