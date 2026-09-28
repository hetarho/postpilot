import { describe, expect, it } from 'vitest'
import { storylineViewOrder } from './view-order'

const paragraphs = [
  { text: '첫 문단', files: ['a.jpg', 'b.jpg'] },
  { text: '둘째 문단', files: [] },
  { text: '셋째 문단', files: ['c.mp4'] },
]

describe('storylineViewOrder', () => {
  it('walks the paragraphs in order, then the taken-out attachments', () => {
    expect(storylineViewOrder(paragraphs, ['d.jpg'], () => true)).toEqual([
      'a.jpg',
      'b.jpg',
      'c.mp4',
      'd.jpg',
    ])
  })

  it('leaves out an attachment with nothing to show', () => {
    expect(storylineViewOrder(paragraphs, ['d.jpg'], (file) => file !== 'b.jpg')).toEqual([
      'a.jpg',
      'c.mp4',
      'd.jpg',
    ])
  })

  it('lists a file once', () => {
    expect(storylineViewOrder(paragraphs, ['a.jpg'], () => true)).toEqual([
      'a.jpg',
      'b.jpg',
      'c.mp4',
    ])
  })
})
