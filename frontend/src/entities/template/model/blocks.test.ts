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
  photoSummaryKey,
  positionAfter,
  repeatPhotoCount,
  reorder,
  toBody,
  type BuilderBlock,
} from './blocks'

// The ceiling the shared fixture declares, so this suite and the grammar suite agree.
const options = { photoRowMax: 4 }
// What an unlabelled legacy position falls back to. The real strings come from i18n; these
// stand in for them, which is the whole point of passing them rather than looking them up.
const legacy = { place: '지도', link: '링크' }

const read = (body: string) => {
  const result = fromBody(body, decode, options, legacy)
  if (!result.ok) throw new Error(`unexpected parse failure: ${JSON.stringify(result.failure)}`)
  return result.blocks
}

describe('builder blocks', () => {
  it('round-trips a body the builder produced, byte for byte', () => {
    const blocks: BuilderBlock[] = [
      { id: 'a', kind: 'write', text: '인트로를 작성합니다.' },
      { id: 'b', kind: 'text', text: '=========================' },
      {
        id: 'd',
        kind: 'repeat',
        children: [
          { id: 'e', kind: 'photo', count: 1 },
          { id: 'f', kind: 'write', text: '이 사진에 대한 설명' },
        ],
      },
      { id: 'g', kind: 'photo', count: 3 },
      { id: 'h', kind: 'write', text: '총평 및 재방문 의사' },
    ]

    const body = toBody(blocks)
    // parse → blocks → serialize is the identity for anything this editor wrote (AC8).
    expect(toBody(read(body))).toBe(body)
    // And the shape survives, not just the bytes.
    expect(read(body).map((block) => block.kind)).toEqual([
      'write',
      'text',
      'repeat',
      'photo',
      'write',
    ])
  })

  // A count of one is written as ABSENCE: emitting count="1" would rewrite every stored body
  // on its next save for no change in meaning.
  it('writes a count only above one, and reads an absent one back as one', () => {
    expect(toBody([{ id: 'a', kind: 'photo', count: 1 }])).toBe('<slot kind="photo"/>')
    expect(toBody([{ id: 'a', kind: 'photo', count: 3 }])).toBe('<slot kind="photo" count="3"/>')
    expect(read('<slot kind="photo"/>')[0]).toMatchObject({ kind: 'photo', count: 1 })
    expect(read('<slot kind="photo" count="4"/>')[0]).toMatchObject({ kind: 'photo', count: 4 })
    // Builder → body → builder holds for a count as it does for everything else.
    const body = toBody([{ id: 'a', kind: 'photo', count: 2 }])
    expect(toBody(read(body))).toBe(body)
  })

  // TEMPLATE-37: the position is retired, but a body that has one must not become unreadable.
  // It opens as FIXED TEXT carrying its label, and the next save writes it back as literal text.
  it('reads a stored place or link position as fixed text carrying its label', () => {
    const blocks = read(
      '<slot kind="place" label="네이버 지도"/>\n<slot kind="link" label="예약"/>',
    )
    expect(blocks).toMatchObject([
      { kind: 'text', text: '네이버 지도' },
      { kind: 'text', text: '예약' },
    ])
    // The save writes literal text: nothing of the retired position survives.
    expect(toBody(blocks)).toBe('네이버 지도\n예약')
  })

  it('falls back to a name when a stored position carried no label', () => {
    expect(read('<slot kind="place"/>\n<slot kind="link"/>')).toMatchObject([
      { kind: 'text', text: '지도' },
      { kind: 'text', text: '링크' },
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

  it('reports a body that does not parse instead of guessing', () => {
    const result = fromBody('<repaet each="photo">\n</repaet>', decode, options, legacy)
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

  it('starts a photo position at one photo', () => {
    expect(newBlock('photo')).toMatchObject({ kind: 'photo', count: 1 })
  })

  it('reports how many photos one iteration of a repeat takes', () => {
    expect(
      repeatPhotoCount({
        id: 'r',
        kind: 'repeat',
        children: [
          { id: 'a', kind: 'photo', count: 2 },
          { id: 'b', kind: 'write', text: '설명' },
          { id: 'c', kind: 'photo', count: 1 },
        ],
      }),
    ).toBe(3)
    expect(repeatPhotoCount({ id: 'r', kind: 'repeat', children: [] })).toBe(0)
  })
})

describe('the collapsed outline', () => {
  const composition: BuilderBlock[] = [
    { id: 'a', kind: 'write', text: '인트로를 씁니다' },
    { id: 'b', kind: 'text', text: '네이버 지도' },
    {
      id: 'c',
      kind: 'repeat',
      children: [
        { id: 'd', kind: 'photo', count: 1 },
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

  it('names a row by the same key as the button that creates it', () => {
    expect(blockKindKey({ id: 'x', kind: 'photo', count: 1 })).toBe('photo')
    expect(blockKindKey({ id: 'y', kind: 'write', text: '' })).toBe('write')
  })

  // One photo reads as a photo; more than one has to say they stand side by side, which is the
  // whole point of the count (TEMPLATE-38).
  it('picks the summary key by whether the photos stand side by side', () => {
    expect(photoSummaryKey(1)).toBe('composition.summary.photo')
    expect(photoSummaryKey(2)).toBe('composition.summary.photoRow')
  })
})

describe('insertion at a position', () => {
  const composition: BuilderBlock[] = [
    { id: 'a', kind: 'write', text: '인트로' },
    {
      id: 'b',
      kind: 'repeat',
      children: [{ id: 'c', kind: 'photo', count: 1 }],
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
    expect(repeat.children.map((child) => child.kind)).toEqual(['photo', 'write'])
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
      children: [{ id: 'c', kind: 'photo', count: 1 }],
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
