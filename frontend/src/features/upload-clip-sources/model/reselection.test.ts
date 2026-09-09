import { expect, it } from 'vitest'
import { clipEditingFixture } from '@/test/clip-editing'
import { ClipSourceMismatchError, matchClipSources } from './reselection'

it('matches exact content fingerprints, not names or order, with missing/unexpected names', () => {
  const sources = clipEditingFixture().sources
  const manifest = sources.map((s) => ({
    ...s,
    filename: 'renamed.mp4',
    contentType: 'video/mp4',
    bytes: 4,
  }))
  expect(() => matchClipSources([...manifest].reverse(), sources)).not.toThrow()
  try {
    matchClipSources([{ ...manifest[0]!, fingerprint: 'wrong', filename: 'wrong.mp4' }], sources)
    throw new Error('expected mismatch')
  } catch (error) {
    expect(error).toBeInstanceOf(ClipSourceMismatchError)
    expect(error).toMatchObject({
      missing: ['source-a.mp4', 'source-b.mp4'],
      unexpected: ['wrong.mp4'],
    })
  }
  expect(() => matchClipSources([manifest[0]!], [sources[0]!])).not.toThrow()
  expect(() => matchClipSources(manifest, [sources[0]!])).toThrow(ClipSourceMismatchError)
})
