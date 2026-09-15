import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { CLIP_COMPOSITION_EXAMPLE } from '../model/composition-guide'
import {
  compositionDesign,
  compositionSkeleton,
  rebuildCompositionSkeleton,
} from './composition-skeleton'
import { compositionDraftTree, compositionOutline } from './composition-author'
import { parseClipComposition } from './composition-parse'

const outro = (source: string) =>
  parseClipComposition(source).elements.find((e) => e.role === 'ending')!
describe('design-first composition skeleton', () => {
  it.each([
    ['a', 'b'],
    ['a', 'e'],
    ['b', 'b'],
    ['b', 'e'],
  ] as const)('creates %s/%s with fixed ordered slots and default intervals', (intro, ending) => {
    const doc = parseClipComposition(compositionSkeleton(intro, ending))
    expect(doc.design).toEqual({ intro, caption: 'bold', outro: ending })
    expect(doc.elements.map((e) => [e.role, e.rows.length, e.startMs, e.endMs])).toEqual([
      ['hook', 2, 0, 2500],
      ['ending', ending === 'b' ? 2 : 3, -3000, 0],
    ])
  })
  it('carries B → E → B rows, bindings, authorship and content bytes without dropping text', () => {
    const content = `\n <scene id='food' scope='scene'><guide> 🥗 A &amp; B </guide></scene>\n`
    const original = compositionSkeleton('b', 'b')
      .replace('<row kind="fixed"/>', '<row kind="fixed">Intro</row>')
      .replace('</clip>', content + '</clip>')
      .replace(
        /(<text[^>]*role="ending"[^>]*>)[\s\S]*?(<\/text>)/,
        '$1<row kind="fixed">First</row><row kind="ai">Second</row>$2',
      )
    const e = rebuildCompositionSkeleton(
      original,
      { intro: 'b', caption: 'bold', outro: 'e' },
      'ending',
    )
    expect(outro(e).rows.map((r) => [r.kind, r.parts.map((p) => p.literal).join('')])).toEqual([
      ['fixed', 'First'],
      ['ai', 'Second'],
      ['fixed', ''],
    ])
    expect(e).toContain(content)
    const b = rebuildCompositionSkeleton(e, { intro: 'b', caption: 'bold', outro: 'b' }, 'ending')
    expect(outro(b).rows).toEqual(outro(original).rows)
    expect(b).toContain(content)
    const full = rebuildCompositionSkeleton(
      e.replace(
        /(<text[^>]*role="ending"[^>]*>)[\s\S]*?(<\/text>)/,
        '$1<row>First</row><row>Second</row><row>Keep third</row>$2',
      ),
      { intro: 'b', caption: 'bold', outro: 'b' },
      'ending',
    )
    expect(full).toContain('Keep third')
    expect(() => parseClipComposition(full)).toThrow('invalid_skeleton')
  })
  it('moves the real restaurant legacy ending to the root and preserves its entire prompt', () => {
    // 0054 removes retired style attributes but intentionally does not restructure this ending.
    const legacy = readFileSync(
      resolve(
        import.meta.dirname,
        '../../../../../backend/internal/platform/db/testdata/restaurant-v2-before-design-selection.xml',
      ),
      'utf8',
    )
      .replace(/ styles="[^"]*"| style="[^"]*"/g, '')
      .replace('version="1"', 'version="1" intro="b" caption="bold" outro="e"')
    expect(compositionDesign(legacy)).toEqual({ intro: 'b', caption: 'bold', outro: 'e' })
    expect(() => parseClipComposition(legacy)).toThrow('invalid_skeleton')
    const original = compositionOutline(compositionDraftTree(legacy)).find(
      (r) => r.node.attributes.id === 'closing_verdict',
    )!.node
    const rebuilt = rebuildCompositionSkeleton(legacy)
    const tree = compositionDraftTree(rebuilt)
    const ending = tree.children.find((n) => n.attributes.id === 'closing_verdict')!
    expect(ending.children[0].children).toMatchObject(
      original.children.map((n) => ({ name: n.name, text: n.text, attributes: n.attributes })),
    )
    expect(outro(rebuilt).rows[0].kind).toBe('ai')
    expect(outro(rebuilt).rows).toHaveLength(3)
    expect(parseClipComposition(rebuilt).sections).toHaveLength(5)
  })
  it('preserves content when inserting missing regions before a whitespace closing tag', () => {
    const source = `<clip version='1'>\n<guide> 🧑‍🍳 Keep &amp; spacing </guide>\n</clip >`
    const rebuilt = rebuildCompositionSkeleton(source)
    expect(rebuilt).toContain('\n<guide> 🧑‍🍳 Keep &amp; spacing </guide>\n')
    expect(parseClipComposition(rebuilt).elements).toHaveLength(2)
  })
  it('pins the copied guide and restaurant XML in the shared parser corpus', () => {
    const corpus = JSON.parse(
      readFileSync(
        resolve(
          import.meta.dirname,
          '../../../../../backend/internal/clip/composition/testdata/corpus.json',
        ),
        'utf8',
      ),
    )
    expect(
      corpus.cases.find((c: { name: string }) => c.name === 'design-first copied guide example')
        .body,
    ).toBe(CLIP_COMPOSITION_EXAMPLE)
    const document = readFileSync(
      resolve(import.meta.dirname, '../../../../../docs/design/clip-restaurant-template-v2.md'),
      'utf8',
    )
      .split('```xml\n')[1]
      .split('\n```')[0]
    expect(
      corpus.cases.find(
        (c: { name: string }) => c.name === 'restaurant v2 design-first pasteable example',
      ).body,
    ).toBe(document)
    expect(parseClipComposition(document).design.outro).toBe('e')
  })
})
