import { describe, expect, it } from 'vitest'
import { decode } from '../lib/grammar'
import { fromBody, newBlock, reorder, toBody, type BuilderBlock } from './blocks'

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
