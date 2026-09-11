import { describe, expect, it } from 'vitest'
import { emptyClipRecipe, normalizeRecipe, validateClipRecipe, type ClipRecipe } from './types'

const valid = (): ClipRecipe => ({
  ...emptyClipRecipe(),
  name: '영상',
  // A template names its category preset, which fixes chip priority, the
  // default CTA and the default accent (CDS-50).
  preset: 'restaurant',
  informationFields: [{ label: '장소', prompt: '어디인가요?' }],
})
describe('clip recipe bounds', () => {
  it('counts Unicode scalars and normalizes labels without rewriting guidance', () => {
    expect(validateClipRecipe({ ...valid(), name: '😀'.repeat(40) }).valid).toBe(true)
    expect(validateClipRecipe({ ...valid(), name: '😀'.repeat(41) }).name).toBe('tooLong')
    expect(
      normalizeRecipe({
        ...valid(),
        cutGuidance: '  exact\n글  ',
        informationFields: [{ label: ' 이름 ', prompt: ' 안내 ' }],
      }),
    ).toMatchObject({
      cutGuidance: '  exact\n글  ',
      informationFields: [{ label: '이름', prompt: '안내' }],
    })
  })
  it.each([
    { name: '' },
    { cutGuidance: '가'.repeat(4001) },
    { copyStyles: [] },
    { copyStyles: ['clean', 'clean'] },
    { copyStyles: ['unknown'] },
    { accent: 'custom' },
    // A save must name one of the five presets, and every approved style set
    // keeps 깔끔하게 (CDS-50, and T102's ValidCopyStyles).
    { preset: '' },
    { preset: 'bakery' },
    { copyStyles: ['memo'] },
    { informationFields: [{ label: '', prompt: 'p' }] },
    { informationFields: [{ label: 'a'.repeat(41), prompt: 'p' }] },
    { informationFields: [{ label: 'a', prompt: 'p'.repeat(201) }] },
    {
      informationFields: [
        { label: ' a ', prompt: 'p' },
        { label: 'a', prompt: 'p' },
      ],
    },
    { informationFields: Array.from({ length: 11 }, (_, i) => ({ label: `${i}`, prompt: 'p' })) },
  ])('rejects invalid recipe %j', (patch) => {
    expect(validateClipRecipe({ ...valid(), ...patch } as ClipRecipe).valid).toBe(false)
  })
})
