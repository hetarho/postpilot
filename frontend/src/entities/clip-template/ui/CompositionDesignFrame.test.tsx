import { cleanup, render } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { CLIP_DESIGN, CLIP_REGIONS, CLIP_RULES } from '@/entities/clip-design/@x/clip-template'
import { parseClipComposition } from '../lib/composition-parse'
import { sampleClipComposition } from '../lib/composition-sample'
import { CompositionDesignFrame } from './CompositionDesignFrame'

afterEach(cleanup)
it.each(['vertical', 'horizontal', 'square'] as const)(
  'draws every preset and the disclosure from export tokens (%s)',
  (ratio) => {
    for (const intro of ['a', 'b'] as const)
      for (const outro of ['b', 'e'] as const) {
        const document = parseClipComposition(
          `<clip version="1" intro="${intro}" caption="bold" outro="${outro}" accent="coral"><text id="opening" kind="fixed" role="hook" basis="output-start"><row>첫 장면</row><row>오늘의 기록</row></text><text id="closing" kind="fixed" role="ending" basis="output-end"><row>평가</row><row>4.5</row>${outro === 'e' ? '<row>다시 올 곳</row>' : ''}</text><text id="info" kind="fixed" role="info" basis="whole"><row role="label">메뉴</row><row role="caption">된장찌개</row></text><text id="ad" kind="fixed" role="badge" basis="whole">광고</text><text id="caption" kind="fixed" role="caption" basis="whole">현재 장면</text></clip>`,
        )
        const timeline = sampleClipComposition(document, 15000, () => '예시')
        const view = render(
          <CompositionDesignFrame
            document={document}
            entries={timeline.elements}
            ratio={ratio}
            label="Preview"
            sampleAI="문구"
            presets={{ intro, outro }}
          />,
        )
        const shape = CLIP_DESIGN.ratios[ratio],
          scale = shape.canvas.height / CLIP_DESIGN.ratios.vertical.canvas.height
        for (const [kind, id] of [
          ['intro', intro],
          ['outro', outro],
        ] as const) {
          const preset = kind === 'intro' ? CLIP_REGIONS.intro[intro] : CLIP_REGIONS.outro[outro]
          // CDS-79: the block keeps its 9:16 spacing and moves as one piece.
          const ys = [...preset.slots.map((s) => s.y), ...preset.rules.map((r) => r.y)]
          const centre = (Math.min(...ys) + Math.max(...ys)) / 2
          const offset = centre * scale - centre
          const group = view.container.querySelector(`[data-region="${kind}"]`)!
          expect(group.getAttribute('data-preset')).toBe(id)
          const slots = group.querySelectorAll('text[data-slot]')
          expect(slots).toHaveLength(preset.slots.length)
          preset.slots.forEach((slot, i) => {
            const type = CLIP_DESIGN.type[slot.type as keyof typeof CLIP_DESIGN.type]
            expect(slots[i]).toHaveAttribute('y', String(slot.y + offset))
            expect(slots[i]).toHaveAttribute(
              'font-size',
              String(slot.type === 'hook' ? shape.hook_size : type.size),
            )
            expect(slots[i]).toHaveAttribute(
              'font-family',
              CLIP_DESIGN.faces[type.face as keyof typeof CLIP_DESIGN.faces],
            )
            expect(slots[i]).toHaveAttribute('fill', CLIP_DESIGN.color.text_white.hex)
          })
          expect(group.querySelectorAll('[data-rule]')).toHaveLength(preset.rules.length)
          preset.rules.forEach((line, i) => {
            const rule = CLIP_RULES[line.kind as keyof typeof CLIP_RULES]
            const actual = group.querySelectorAll('[data-rule]')[i]
            expect(actual).toHaveAttribute('y', String(line.y + offset))
            expect(actual).toHaveAttribute('width', String(rule.w))
            expect(actual).toHaveAttribute('height', String(rule.h))
            expect(actual).toHaveAttribute('fill', CLIP_DESIGN.color.text_white.hex)
          })
          expect(group.querySelector('tspan')).toBeNull()
        }
        const badge = view.container.querySelector('[data-disclosure]')!
        expect(badge).toHaveAttribute('height', '68')
        expect(badge).toHaveAttribute('rx', String(CLIP_DESIGN.spacing.radius_chip))
        expect(view.container.querySelectorAll('[rx]')).toHaveLength(1)
        const info = view.container.querySelector('[data-role="info"]')!
        expect(info.querySelectorAll('text')[0]).toHaveAttribute(
          'letter-spacing',
          String(36 * 0.08),
        )
        expect(info.querySelectorAll('text')[0]).toHaveAttribute('fill-opacity', '0.72')
        expect(info.querySelectorAll('text')[1]).toHaveAttribute('font-size', '44')
        expect(info.querySelector('rect')).toBeNull()
        expect(view.container.querySelector('[data-role="caption"] tspan')).toHaveAttribute(
          'fill',
          CLIP_DESIGN.accent.coral,
        )
        view.unmount()
      }
  },
)

it('moves an automatic caption to the alternative anchor around visible outro slots', () => {
  const previous = Object.getOwnPropertyDescriptor(SVGElement.prototype, 'getBBox')
  Object.defineProperty(SVGElement.prototype, 'getBBox', {
    configurable: true,
    value(this: SVGTextElement) {
      const size = Number(this.getAttribute('font-size'))
      return {
        x: 0,
        y: -size * 0.8,
        width: (this.textContent?.length ?? 0) * size * 0.5,
        height: size * 0.8,
      }
    },
  })
  try {
    const document = parseClipComposition(
      '<clip version="1" intro="b" caption="bold" outro="e"><text id="intro" role="hook" kind="fixed" basis="output-start"/><text id="outro" role="ending" kind="fixed" basis="output-end"><row>개인 점수</row><row>4.5</row></text><text id="caption" role="caption" kind="fixed" basis="whole">오늘의 한 끼</text></clip>',
    )
    const timeline = sampleClipComposition(document, 15000, () => '예시')
    const view = render(
      <CompositionDesignFrame
        document={document}
        entries={timeline.elements}
        ratio="horizontal"
        label="Preview"
        sampleAI="문구"
      />,
    )
    expect(view.container.querySelector('[data-role="caption"]')).toHaveAttribute(
      'data-position',
      CLIP_REGIONS.caption.bold.anchor_alt,
    )
  } finally {
    if (previous) Object.defineProperty(SVGElement.prototype, 'getBBox', previous)
    else Reflect.deleteProperty(SVGElement.prototype, 'getBBox')
  }
})

// CLIP-147: a line the chosen preset cannot draw is left out of the picture the
// same way the render leaves it out, rather than breaking the preview.
it('draws only the lines the chosen preset holds', () => {
  const document = parseClipComposition(
    '<clip version="1"><text id="opening" kind="fixed" role="hook"><row>첫 줄</row><row>둘째 줄</row><row>셋째 줄</row></text></clip>',
  )
  const timeline = sampleClipComposition(document, 15000, () => '예시')
  const view = render(
    <CompositionDesignFrame
      document={document}
      entries={timeline.elements}
      ratio="vertical"
      label="Preview"
      sampleAI="문구"
    />,
  )
  const group = view.container.querySelector('[data-region="intro"]')!
  expect(group.querySelectorAll('text[data-slot]')).toHaveLength(2)
})

// CDS-79: the preview must land on the renderer's own baselines, not on its own
// arithmetic. These are the y values the Go region goldens carry for the two
// 1080-high canvases; 9:16 keeps the preset's authored values unchanged.
it.each([
  ['vertical', 'b', 'e', [846, 960, 1010, 1090], [836, 980, 1030, 1110]],
  ['horizontal', 'b', 'e', [422.5, 536.5, 586.5, 666.5], [410.3125, 554.3125, 604.3125, 684.3125]],
  ['square', 'b', 'e', [422.5, 536.5, 586.5, 666.5], [410.3125, 554.3125, 604.3125, 684.3125]],
] as const)(
  'places region parts on the renderer baselines (%s)',
  (ratio, intro, outro, wantIntro, wantOutro) => {
    const document = parseClipComposition(
      `<clip version="1" intro="${intro}" caption="bold" outro="${outro}"><text id="opening" kind="fixed" role="hook" basis="output-start"><row>첫 장면</row><row>오늘의 기록</row></text><text id="closing" kind="fixed" role="ending" basis="output-end"><row>평가</row><row>4.5</row><row>다시 올 곳</row></text></clip>`,
    )
    const timeline = sampleClipComposition(document, 15000, () => '예시')
    const view = render(
      <CompositionDesignFrame
        document={document}
        entries={timeline.elements}
        ratio={ratio}
        label="Preview"
        sampleAI="문구"
      />,
    )
    for (const [kind, want] of [
      ['intro', wantIntro],
      ['outro', wantOutro],
    ] as const) {
      const group = view.container.querySelector(`[data-region="${kind}"]`)!
      const painted = [...group.querySelectorAll('text[data-slot], [data-rule]')]
        .map((node) => Number(node.getAttribute('y')))
        .sort((a, b) => a - b)
      expect(painted).toEqual([...want].sort((a, b) => a - b))
    }
  },
)
