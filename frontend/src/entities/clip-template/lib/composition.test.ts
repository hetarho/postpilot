/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'
import {
  CompositionProblem,
  type ClipComposition,
  type CompositionInputs,
  type CompositionNode,
  type CompositionTimeline,
} from '../model/composition'
import {
  compositionMilliseconds,
  parseClipComposition,
  replaceCompositionNode,
  replaceCompositionSpan,
} from './composition-parse'
import { serializeCompositionNode } from './composition-xml'
import { resolveClipComposition } from './composition-resolve'

const corpus = JSON.parse(
  readFileSync(
    resolve(
      import.meta.dirname,
      '../../../../../backend/internal/clip/composition/testdata/corpus.json',
    ),
    'utf8',
  ),
) as {
  limits: typeof CLIP_COMPOSITION_LIMITS
  cases: {
    name: string
    body: string
    error?: { reason: string; elementId: string; line: number }
    summary?: unknown
    rootSpan?: unknown
    inputs?: CompositionInputs
    durationMs?: number
    resolvedCount?: number
    resolved?: unknown
    maxExpandedBytes?: number
  }[]
}
const summary = (d: ClipComposition) => ({
  styles: d.styles,
  accent: d.accent,
  pace: d.pace,
  fields: d.fields.map(({ id, group, label, prompt, required }) => ({
    id,
    group,
    label,
    prompt,
    required,
  })),
  groups: d.groups,
  sections: d.sections.map(({ id, scope, repeat }) => ({ id, scope, repeat })),
  elements: d.elements.map((e) => e.id),
  guidance: d.guidance,
})
const resolution = (t: CompositionTimeline) =>
  t.elements.map(({ instanceId, text, startMs, endMs, authoredTiming, facts, rows }) => ({
    instanceId,
    text,
    startMs,
    endMs,
    authoredTiming,
    facts,
    rows,
  }))

describe('shared portable composition contract', () => {
  it('addresses repeated field names by group and guides by exact node span', () => {
    const source =
      '<clip version="1"><guide>keep</guide><group id="a"><field id="price" label="A"/></group><group id="b"><field id="price" label="B"/></group></clip>'
    let d = parseClipComposition(source)
    const replacement: CompositionNode = {
      name: 'field',
      attributes: { id: 'price', label: 'changed' },
      children: [],
      text: '',
      span: d.root.span,
    }
    d = replaceCompositionNode(d, 'a.price', replacement)
    expect(d.fields.map((f) => f.label)).toEqual(['changed', 'B'])
    const guide = d.root.children[0]
    d = replaceCompositionSpan(d, guide.span, {
      ...guide,
      children: [{ ...guide.children[0], text: 'new guidance' }],
    })
    expect(d.guidance).toEqual(['new guidance'])
    expect(() =>
      replaceCompositionSpan(d, { start: 1, end: 3, line: 1 }, replacement),
    ).toThrowError('unknown_element')
  })
  it('uses the same finite configuration as the backend corpus', () => {
    expect(CLIP_COMPOSITION_LIMITS).toEqual(corpus.limits)
  })
  for (const c of corpus.cases)
    it(c.name, () => {
      let document: ClipComposition | undefined,
        timeline: CompositionTimeline | undefined,
        caught: unknown
      try {
        document = parseClipComposition(c.body)
        if (c.inputs)
          timeline = resolveClipComposition(document, c.inputs, c.maxExpandedBytes ?? 1 << 20)
      } catch (error) {
        caught = error
      }
      if (c.error) {
        expect(caught).toBeInstanceOf(CompositionProblem)
        const error = caught as CompositionProblem
        expect({ elementId: error.elementId, line: error.line, reason: error.reason }).toEqual(
          c.error,
        )
        return
      }
      expect(caught).toBeUndefined()
      const d = document!
      expect(d.source).toBe(c.body)
      if (c.rootSpan) expect(d.root.span).toEqual(c.rootSpan)
      if (c.summary) expect(summary(d)).toEqual(c.summary)
      if (c.inputs) {
        expect(timeline!.durationMs).toBe(c.durationMs)
        if (c.resolved !== undefined) expect(resolution(timeline!)).toEqual(c.resolved)
        if (c.resolvedCount !== undefined) expect(timeline!.elements).toHaveLength(c.resolvedCount)
      }
      const canonical = serializeCompositionNode(d.root),
        again = parseClipComposition(canonical)
      expect(serializeCompositionNode(again.root)).toBe(canonical)
      if (c.inputs)
        expect(
          resolution(resolveClipComposition(again, c.inputs, c.maxExpandedBytes ?? 1 << 20)),
        ).toEqual(resolution(timeline!))
    })
  it('changes only the selected subtree, preserving original Unicode, quotes and whitespace', () => {
    const source =
      " \n<clip version='1'>\n <guide>🧑‍🍳 keep &amp; spacing</guide>\n <text id='copy' kind='fixed' role='caption' basis='whole'>old</text>\n</clip>\n"
    const d = parseClipComposition(source),
      node = d.root.children.find((n) => n.name === 'text')!
    const replacement: CompositionNode = {
      ...node,
      children: [
        { name: '#text', attributes: {}, children: [], text: '새 <문구>', span: node.span },
      ],
    }
    const updated = replaceCompositionNode(d, 'copy', replacement),
      chars = Array.from(source)
    expect(updated.source.startsWith(chars.slice(0, node.span.start).join(''))).toBe(true)
    expect(updated.source.endsWith(chars.slice(node.span.end).join(''))).toBe(true)
    expect(updated.elements[0].parts[0].literal).toBe('새 <문구>')
  })
  it('parses exact milliseconds and rejects lossy or unsupported numbers', () => {
    for (const [s, expected] of [
      ['0.001', 1],
      ['01.020', 1020],
      ['-2.5', -2500],
      ['90', 90000],
      ['90.001', null],
      ['1e2', null],
      ['NaN', null],
      ['0.0001', null],
      ['999999999999999999999999', null],
    ] as const)
      expect(compositionMilliseconds(s, 90000)).toBe(expected)
  })
  it('does not accept malformed Unicode or inherit an answer from the object prototype', () => {
    expect(() => parseClipComposition('<clip version="1">\ud800</clip>')).toThrowError(
      'invalid_unicode',
    )
    const d = parseClipComposition(
      '<clip version="1"><field id="constructor" label="값" required="true"/></clip>',
    )
    expect(() =>
      resolveClipComposition(d, { values: {}, items: {}, cuts: [] }, 10000),
    ).toThrowError('required_binding')
  })
})
