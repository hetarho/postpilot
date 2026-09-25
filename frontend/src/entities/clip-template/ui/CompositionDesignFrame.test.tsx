import { cleanup, render } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import {
  CLIP_DESIGN,
  CLIP_REGIONS,
  clipLayoutRegion,
} from '@/entities/clip-design/@x/clip-template'
import fixture from '@/entities/clip-design/model/region-layouts.fixture.json'
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
        const rows = {
          intro: ['첫 장면', '오늘의 기록'],
          outro: outro === 'e' ? ['평가', '4.5', '다시 올 곳'] : ['평가', '4.5'],
        }
        for (const [kind, id] of [
          ['intro', intro],
          ['outro', outro],
        ] as const) {
          // CDS-86, CDS-87: the preview draws the renderer's own block layout.
          const block = clipLayoutRegion(kind, id, ratio, rows[kind])!
          const group = view.container.querySelector(`[data-region="${kind}"]`)!
          expect(group.getAttribute('data-preset')).toBe(id)
          const texts = group.querySelectorAll('text[data-slot]')
          const lines = block.slots.flatMap((slot) => slot.lines.map((line) => ({ slot, line })))
          expect(texts).toHaveLength(lines.length)
          lines.forEach(({ slot, line }, i) => {
            expect(texts[i]).toHaveAttribute('y', String(line.baseline))
            expect(texts[i]).toHaveAttribute('font-size', String(line.size))
            expect(texts[i]).toHaveAttribute(
              'font-family',
              CLIP_DESIGN.faces[slot.type.face as keyof typeof CLIP_DESIGN.faces],
            )
            expect(texts[i]).toHaveAttribute('fill', CLIP_DESIGN.color.text_white.hex)
          })
          const painted = group.querySelectorAll('[data-rule]')
          expect(painted).toHaveLength(block.rules.length)
          block.rules.forEach((rule, i) => {
            expect(painted[i]).toHaveAttribute('y', String(rule.box.y))
            expect(painted[i]).toHaveAttribute('width', String(rule.box.width))
            expect(painted[i]).toHaveAttribute('height', String(rule.box.height))
            expect(painted[i]).toHaveAttribute('fill', CLIP_DESIGN.color.text_white.hex)
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
    // A wrapped outro B hook covers the caption's default anchor on 16:9 and
    // leaves its alternative free; check that the layout really says so first.
    const hook = '연남동 골목에서 다시 가고 싶은 숯불 한우 불판 맛집 1순위'
    const block = clipLayoutRegion('outro', 'b', 'horizontal', [hook, ''])!
    const shape = CLIP_DESIGN.ratios.horizontal
    const captionHeight = 72 * 0.8 + CLIP_DESIGN.spacing.stroke_text
    const covers = (anchor: number) =>
      block.slots.some(
        (s) =>
          s.box.y < anchor + captionHeight / 2 &&
          s.box.y + s.box.height > anchor - captionHeight / 2,
      )
    expect(block.slots[0].lines).toHaveLength(2)
    expect(covers(shape.anchor.upper_mid)).toBe(true)
    expect(covers(shape.anchor.lower_mid)).toBe(false)
    const document = parseClipComposition(
      `<clip version="1"><text id="outro" role="ending" kind="fixed" basis="output-end"><row>${hook}</row></text><text id="caption" role="caption" kind="fixed" basis="whole">오늘의 한 끼</text></clip>`,
    )
    const timeline = sampleClipComposition(document, 15000, () => '예시')
    const view = render(
      <CompositionDesignFrame
        document={document}
        entries={timeline.elements}
        ratio="horizontal"
        label="Preview"
        sampleAI="문구"
        presets={{ intro: 'b', outro: 'b' }}
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

// CDS-79, CDS-86, CDS-87: the preview lands on the renderer's own baselines —
// the Go layout of every preset, ratio and row set, written by the Go design
// tests — not on arithmetic of its own.
it('places region parts where the renderer lays them out', () => {
  for (const c of fixture) {
    const role = c.kind === 'intro' ? 'hook' : 'ending'
    const rows = c.rows.map((row) => `<row>${row}</row>`).join('')
    const document = parseClipComposition(
      `<clip version="1"><text id="region" kind="fixed" role="${role}">${rows}</text></clip>`,
    )
    const timeline = sampleClipComposition(document, 15000, () => '예시')
    const view = render(
      <CompositionDesignFrame
        document={document}
        entries={timeline.elements}
        ratio={c.ratio as 'vertical' | 'horizontal' | 'square'}
        label="Preview"
        sampleAI="문구"
        presets={{
          intro: c.kind === 'intro' ? (c.id as 'a' | 'b') : 'b',
          outro: c.kind === 'outro' ? (c.id as 'b' | 'e') : 'e',
        }}
      />,
    )
    const group = view.container.querySelector(`[data-region="${c.kind}"]`)
    const want = [
      ...c.slots.flatMap((slot) => slot.lines.map((line) => line.baseline)),
      ...c.rules.map((rule) => rule.y),
    ]
    const painted = [...(group?.querySelectorAll('text[data-slot], [data-rule]') ?? [])].map(
      (node) => Number(node.getAttribute('y')),
    )
    expect(painted.length, JSON.stringify(c.rows)).toBe(want.length)
    painted
      .sort((a, b) => a - b)
      .forEach((y, i) =>
        expect(Math.abs(y - [...want].sort((a, b) => a - b)[i])).toBeLessThan(0.01),
      )
    view.unmount()
  }
})
