import { describe, expect, it } from 'vitest'
import { decode } from '../lib/grammar'
import {
  blockKindKey,
  blockSummary,
  canInsert,
  endPosition,
  fromBody,
  insertAt,
  newBlock,
  outline,
  positionAfter,
  reorder,
  toBody,
  type BuilderBlock,
} from './blocks'

const read = (body: string) => {
  const result = fromBody(body, decode)
  if (!result.ok) throw new Error(`unexpected parse failure: ${JSON.stringify(result.failure)}`)
  return result.blocks
}

describe('builder blocks', () => {
  it('round-trips a body the builder produced, byte for byte', () => {
    const blocks: BuilderBlock[] = [
      { id: 'a', kind: 'write', text: '인트로를 작성합니다.' },
      { id: 'b', kind: 'text', text: '=========================' },
      { id: 'c', kind: 'slot', slotKind: 'place', label: '네이버 지도' },
      {
        id: 'd',
        kind: 'repeat',
        children: [
          { id: 'e', kind: 'slot', slotKind: 'photo', label: '' },
          { id: 'f', kind: 'write', text: '이 사진에 대한 설명' },
        ],
      },
      { id: 'g', kind: 'write', text: '총평 및 재방문 의사' },
    ]

    const body = toBody(blocks)
    // parse → blocks → serialize is the identity for anything this editor wrote (AC8).
    expect(toBody(read(body))).toBe(body)
    // And the shape survives, not just the bytes.
    expect(read(body).map((block) => block.kind)).toEqual([
      'write',
      'text',
      'slot',
      'repeat',
      'write',
    ])
  })

  it('drops the separator between two tags rather than showing it as an empty row', () => {
    const blocks = read('<write>a</write>\n<write>b</write>')
    expect(blocks.map((block) => block.kind)).toEqual(['write', 'write'])
  })

  it('keeps literal prose between tags as its own row', () => {
    const blocks = read('<write>a</write>\n=====\n<write>b</write>')
    expect(blocks.map((block) => block.kind)).toEqual(['write', 'text', 'write'])
    expect(blocks[1]).toMatchObject({ kind: 'text', text: '=====' })
  })

  it('escapes a literal that would otherwise start a tag, and reads it back unescaped', () => {
    const body = toBody([{ id: 'a', kind: 'text', text: '<write> 라고 씁니다' }])
    const blocks = read(body)
    expect(blocks[0]).toMatchObject({ kind: 'text', text: '<write> 라고 씁니다' })
  })

  it('keeps a quote in a slot label instead of emitting a body its own parser refuses', () => {
    // The attribute value is quoted, so an unescaped `"` used to end it early: the builder
    // produced `label="네이버 "지도"/>`, the parser said malformed_tag, and the editor fell into
    // source mode on a valid keystroke.
    const body = toBody([{ id: 'a', kind: 'slot', slotKind: 'place', label: '네이버 "지도"' }])
    expect(body).toBe('<slot kind="place" label="네이버 &quot;지도&quot;"/>')
    expect(read(body)[0]).toMatchObject({ kind: 'slot', label: '네이버 "지도"' })
    expect(toBody(read(body))).toBe(body)
  })

  it('omits an empty slot label instead of writing an empty attribute', () => {
    const body = toBody([{ id: 'a', kind: 'slot', slotKind: 'link', label: '  ' }])
    expect(body).toBe('<slot kind="link"/>')
  })

  it('reports a body that does not parse instead of guessing', () => {
    const result = fromBody('<repaet each="photo">\n</repaet>', decode)
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.failure).toEqual({ line: 1, reason: 'unknown_tag' })
  })

  it('moves a block across the list in one splice, not a chain of swaps', () => {
    const items = ['a', 'b', 'c', 'd']
    expect(reorder(items, 0, 3)).toEqual(['b', 'c', 'd', 'a'])
    expect(reorder(items, 3, 0)).toEqual(['d', 'a', 'b', 'c'])
    // Out-of-range targets clamp rather than dropping the item.
    expect(reorder(items, 1, 99)).toEqual(['a', 'c', 'd', 'b'])
  })

  it('gives every new block its own identity so two identical rows stay two rows', () => {
    expect(newBlock('write').id).not.toBe(newBlock('write').id)
  })

  it('maps a palette slot kind onto a slot block', () => {
    expect(newBlock('photo')).toMatchObject({ kind: 'slot', slotKind: 'photo' })
    expect(newBlock('place')).toMatchObject({ kind: 'slot', slotKind: 'place' })
    expect(newBlock('link')).toMatchObject({ kind: 'slot', slotKind: 'link' })
  })
})

describe('the collapsed outline', () => {
  const composition: BuilderBlock[] = [
    { id: 'a', kind: 'write', text: '인트로를 씁니다' },
    { id: 'b', kind: 'slot', slotKind: 'place', label: '네이버 지도' },
    {
      id: 'c',
      kind: 'repeat',
      children: [
        { id: 'd', kind: 'slot', slotKind: 'photo', label: '' },
        { id: 'e', kind: 'write', text: '이 사진에 대한 설명' },
      ],
    },
  ]

  // A5: the rows ARE the outline — parents before their children, one indent step, no third level.
  it('flattens parents before their children with one indent step', () => {
    expect(
      outline(composition).map((row) => [row.block.id, row.depth, row.parentId, row.index]),
    ).toEqual([
      ['a', 0, null, 0],
      ['b', 0, null, 1],
      ['c', 0, null, 2],
      ['d', 1, 'c', 0],
      ['e', 1, 'c', 1],
    ])
  })

  // A9: a summary is the block's own text. Nothing of the grammar reaches a row.
  it('summarizes a block by its own text and never by its markup', () => {
    const summaries = outline(composition).map((row) => blockSummary(row.block))
    expect(summaries).toEqual(['인트로를 씁니다', '네이버 지도', '', '', '이 사진에 대한 설명'])
    for (const summary of summaries) {
      expect(summary).not.toMatch(/[<>]/)
    }
  })

  it('collapses newlines so a multi-line literal still reads as one line', () => {
    expect(blockSummary({ id: 'x', kind: 'text', text: '  첫 줄\n\n둘째 줄  ' })).toBe(
      '첫 줄 둘째 줄',
    )
  })

  it('names a slot by its kind, not by the word slot', () => {
    expect(blockKindKey({ id: 'x', kind: 'slot', slotKind: 'photo', label: '' })).toBe('photo')
    expect(blockKindKey({ id: 'y', kind: 'write', text: '' })).toBe('write')
  })
})

describe('insertion at a position', () => {
  const composition: BuilderBlock[] = [
    { id: 'a', kind: 'write', text: '인트로' },
    {
      id: 'b',
      kind: 'repeat',
      children: [{ id: 'c', kind: 'slot', slotKind: 'photo', label: '' }],
    },
  ]

  // A7: the toolbar's block lands where the screen said it would.
  it('inserts at the top level and inside a repeat', () => {
    const top = insertAt(composition, { parentId: null, index: 1 }, 'note')
    expect(top.blocks.map((block) => block.kind)).toEqual(['write', 'note', 'repeat'])
    expect(top.inserted?.kind).toBe('note')

    const inside = insertAt(composition, { parentId: 'b', index: 1 }, 'write')
    const repeat = inside.blocks[1]
    if (repeat.kind !== 'repeat') throw new Error('the repeat moved')
    expect(repeat.children.map((child) => child.kind)).toEqual(['slot', 'write'])
    // The other blocks are untouched, so the insertion cannot disturb the outline around it.
    expect(inside.blocks[0]).toBe(composition[0])
  })

  it('appends at the end position when nothing has been touched', () => {
    const end = endPosition(composition)
    expect(end).toEqual({ parentId: null, index: 2 })
    expect(insertAt(composition, end, 'text').blocks.map((b) => b.kind)).toEqual([
      'write',
      'repeat',
      'text',
    ])
  })

  // A8: the grammar forbids a repeat inside a repeat, and the model is what enforces it — the
  // palette merely hides the button.
  it('refuses a repeat inside a repeat and changes nothing', () => {
    expect(canInsert('repeat', { parentId: 'b', index: 0 })).toBe(false)
    const refused = insertAt(composition, { parentId: 'b', index: 0 }, 'repeat')
    expect(refused.inserted).toBeNull()
    expect(toBody(refused.blocks)).toBe(toBody(composition))
  })

  // Falling back to the end would put the block somewhere other than where the screen said.
  it('inserts nothing when the parent is gone', () => {
    const orphaned = insertAt(composition, { parentId: 'nope', index: 0 }, 'write')
    expect(orphaned.inserted).toBeNull()
    expect(toBody(orphaned.blocks)).toBe(toBody(composition))
  })
})

describe('the position a touched row leaves behind', () => {
  const composition: BuilderBlock[] = [
    { id: 'a', kind: 'write', text: '인트로' },
    {
      id: 'b',
      kind: 'repeat',
      children: [{ id: 'c', kind: 'slot', slotKind: 'photo', label: '' }],
    },
  ]

  it('aims after a plain row, and inside a repeat when the row is its child', () => {
    expect(positionAfter(composition, 'a')).toEqual({ parentId: null, index: 1 })
    expect(positionAfter(composition, 'c')).toEqual({ parentId: 'b', index: 1 })
  })

  // Touching the repeat itself aims INSIDE it: a repeat exists to hold children.
  it('aims inside a repeat when the repeat itself is touched', () => {
    expect(positionAfter(composition, 'b')).toEqual({ parentId: 'b', index: 1 })
  })

  it('has no position for a block that is gone', () => {
    expect(positionAfter(composition, 'nope')).toBeNull()
  })
})
