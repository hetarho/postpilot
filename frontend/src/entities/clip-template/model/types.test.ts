import { describe, expect, it } from 'vitest'
import { emptyClipRecipe, normalizeRecipe, validateClipRecipe, type ClipRecipe } from './types'

const valid = (): ClipRecipe => ({
  ...emptyClipRecipe(),
  name: '영상',
  compositionBody: '<clip version="1"/>',
})
describe('clip recipe bounds', () => {
  it('counts Unicode scalars and trims the name without rewriting the body', () => {
    expect(validateClipRecipe({ ...valid(), name: '😀'.repeat(40) }).valid).toBe(true)
    expect(validateClipRecipe({ ...valid(), name: '😀'.repeat(41) }).name).toBe('tooLong')
    const body = '\n<clip version="1"/>\n'
    expect(
      normalizeRecipe({ ...emptyClipRecipe(), name: ' 이름 ', compositionBody: body }),
    ).toEqual({ ...emptyClipRecipe(), name: '이름', compositionBody: body })
  })
  it.each([{ name: '' }, { name: '   ' }, { compositionBody: '' }, { compositionBody: '<clip/>' }])(
    'rejects invalid recipe %j',
    (patch) => {
      expect(validateClipRecipe({ ...valid(), ...patch }).valid).toBe(false)
    },
  )
})
