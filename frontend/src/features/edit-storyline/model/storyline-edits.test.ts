import { describe, expect, it } from 'vitest'
import { takenOutFiles, withFileIn, withText, withoutFile } from './storyline-edits'

const PARAGRAPHS = [
  { text: '가게 앞', files: ['a.jpg', 'clip.mp4'] },
  { text: '커피', files: ['b.jpg'] },
]

describe('storyline edits', () => {
  it('replaces one paragraph’s text', () => {
    expect(withText(PARAGRAPHS, 1, '라떼')).toEqual([
      PARAGRAPHS[0],
      { text: '라떼', files: ['b.jpg'] },
    ])
  })

  it('moves a file to the end of another paragraph, and a move in place changes nothing', () => {
    expect(withFileIn(PARAGRAPHS, 'a.jpg', 1)).toEqual([
      { text: '가게 앞', files: ['clip.mp4'] },
      { text: '커피', files: ['b.jpg', 'a.jpg'] },
    ])
    expect(withFileIn(PARAGRAPHS, 'a.jpg', 0)).toEqual(PARAGRAPHS)
  })

  it('takes a file out, and puts a taken-out file back', () => {
    const out = withoutFile(PARAGRAPHS, 'b.jpg')
    expect(out[1]!.files).toEqual([])
    expect(withFileIn(out, 'b.jpg', 0)[0]!.files).toEqual(['a.jpg', 'clip.mp4', 'b.jpg'])
  })

  it('lists what the storyline was made with and no paragraph holds, never an added file', () => {
    const server = { paragraphs: PARAGRAPHS, takenOutFiles: ['c.jpg'] }
    expect(takenOutFiles(server, PARAGRAPHS)).toEqual(['c.jpg'])
    expect(takenOutFiles(server, withoutFile(PARAGRAPHS, 'a.jpg'))).toEqual(['a.jpg', 'c.jpg'])
    expect(takenOutFiles(server, withFileIn(PARAGRAPHS, 'c.jpg', 1))).toEqual([])
  })
})
