import { expect, it } from 'vitest'
import { compositionNode as node, compositionLiteral as literal } from './composition-author'
import { serializeCompositionNode } from './composition-xml'
import { parseClipComposition } from './composition-parse'
import { resolveClipComposition } from './composition-resolve'
import type { CompositionInputs } from '../model/composition'

const pasted = `<clip version="1" styles="clean">
<group id="menu"><field id="name" label="메뉴" required="true"/><field id="price" label="가격"/></group>
<repeat for="menu"><scene id="dish" scope="item">
<text id="name" kind="fixed" role="caption" basis="cut"><value field="menu.name"/></text>
<text id="price" kind="fixed" role="info" basis="cut"><value field="menu.price"/></text>
</scene></repeat><text id="end" kind="fixed" role="caption" basis="output-end" start="-2" end="0">  직접 기록  </text></clip>`

// The same tree constructors and serializer used by CompositionBuilder's controls.
const built = serializeCompositionNode(
  node('clip', { version: '1', styles: 'clean' }, [
    node('group', { id: 'menu' }, [
      node('field', { id: 'name', label: '메뉴', required: 'true' }),
      node('field', { id: 'price', label: '가격', required: 'false' }),
    ]),
    node('repeat', { for: 'menu' }, [
      node('scene', { id: 'dish', scope: 'item' }, [
        node('text', { id: 'name', kind: 'fixed', role: 'caption', basis: 'cut' }, [
          node('value', { field: 'menu.name' }),
        ]),
        node('text', { id: 'price', kind: 'fixed', role: 'info', basis: 'cut' }, [
          node('value', { field: 'menu.price' }),
        ]),
      ]),
    ]),
    node(
      'text',
      { id: 'end', kind: 'fixed', role: 'caption', basis: 'output-end', start: '-2', end: '0' },
      [literal('  직접 기록  ')],
    ),
  ]),
)
const inputs = (): CompositionInputs => ({
  values: {},
  items: {
    menu: [
      { id: 'sea', values: { name: '해물라면', price: '12,000원' } },
      { id: 'cheese', values: { name: '치즈라면', price: '$12 per serving' } },
    ],
  },
  cuts: [
    {
      id: 'sea-cut',
      sourceId: 'source',
      sectionId: 'dish',
      groupId: 'menu',
      itemId: 'sea',
      startMs: 0,
      endMs: 7600,
      transitionMs: 0,
    },
    {
      id: 'cheese-cut',
      sourceId: 'source',
      sectionId: 'dish',
      groupId: 'menu',
      itemId: 'cheese',
      startMs: 7600,
      endMs: 15200,
      transitionMs: 200,
    },
  ],
})
const visible = (source: string, input: CompositionInputs) => {
  const document = parseClipComposition(source)
  expect(document.fields.filter((f) => f.required).map((f) => `${f.group}.${f.id}`)).toEqual([
    'menu.name',
  ])
  return resolveClipComposition(document, input, 262144).elements.map((e) => ({
    id: e.instanceId,
    text: e.text,
    start: e.startMs,
    end: e.endMs,
    item: e.itemId,
    role: e.element.role,
    facts: e.facts,
  }))
}

it('freezes equivalent visible semantics from pasted and builder-authored multi-menu templates', () => {
  const a = visible(pasted, inputs()),
    b = visible(built, inputs())
  expect(a).toEqual(b)
  expect([...new Set(a.map((e) => e.role))].sort()).toEqual(['caption', 'info'])
  expect(
    a.filter((e) => e.role === 'info').map((e) => [e.item, e.text, e.facts[0].itemId]),
  ).toEqual([
    ['sea', '12,000원', 'sea'],
    ['cheese', '$12 per serving', 'cheese'],
  ])
})

it('omits optional price without a placeholder and keeps exact end-relative text after duration change', () => {
  const input = inputs()
  delete input.items.menu[0].values.price
  input.cuts[1].endMs -= 1000
  const result = visible(built, input)
  expect(result.filter((e) => e.role === 'info').map((e) => e.item)).toEqual(['cheese'])
  expect(result.find((e) => e.id === 'end')).toMatchObject({
    text: '  직접 기록  ',
    start: 12000,
    end: 14000,
  })
  expect(result).toEqual(visible(pasted, input))
})
