import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { ClipProjectSchema } from '@/shared/api'
import { toClipProject } from './clip-project'
import { compositionInputsToProto, toProjectComposition } from './composition'
import { projectDraft } from '../model/types'

describe('owned composition transport', () => {
  it('keeps exact source, distinct item IDs and field values through project drafts', () => {
    const value = create(ClipProjectSchema, {
      ratio: 'vertical',
      composition: {
        snapshot: { version: 1, body: '\n<clip version="1"/>\n', templateId: 'template' },
        inputs: {
          values: { a: '  exact 🥣\n', b: 'second answer' },
          items: {
            menu: {
              items: [
                { id: 'a', values: { price: '10,000원' } },
                { id: 'b', values: { price: '20,000원' } },
              ],
            },
          },
          associations: [
            {
              groupId: 'menu',
              itemId: 'b',
              sourceId: 'source',
              fingerprint: 'sha',
              startMs: 123,
              endMs: 456,
            },
          ],
        },
      },
    })
    const p = toClipProject(value)
    expect(p.composition?.snapshot.body).toBe(value.composition?.snapshot?.body)
    const draft = projectDraft(p)
    expect(draft.compositionInputs).toEqual(p.composition?.inputs)
    const wire = compositionInputsToProto(draft.compositionInputs!)
    expect(wire.items.menu.items[1].values.price).toBe('20,000원')
    expect(wire.values.a).toBe('  exact 🥣\n')
    wire.items.menu.items[1].values.price = 'changed'
    expect(draft.compositionInputs?.items.menu[1].values.price).toBe('20,000원')
    expect(toProjectComposition(undefined)).toBeUndefined()
  })
  it('rejects unknown snapshot versions and preserves explicit empty input patches', () => {
    const p = create(ClipProjectSchema, {
      composition: { snapshot: { version: 99, body: 'x' }, inputs: {} },
    })
    expect(() => toProjectComposition(p.composition)).toThrow('Invalid clip composition')
    expect(compositionInputsToProto({ values: {}, items: {}, associations: [] })).toEqual({
      values: {},
      items: {},
      associations: [],
    })
  })
})
