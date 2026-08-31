import { describe, expect, it } from 'vitest'
import { editorSteps, stepForStatus } from './steps'

describe('stepForStatus', () => {
  it('maps each shipped status to its step', () => {
    expect(stepForStatus('draft')).toBe('generate')
    expect(stepForStatus('review')).toBe('refine')
    expect(stepForStatus('finalized')).toBe('finish')
  })

  // A status a later plan adds must not land the editor on an empty panel with no way back.
  it('falls back to the first step for an unknown status', () => {
    expect(stepForStatus('archived')).toBe('generate')
    expect(stepForStatus('')).toBe('generate')
  })

  it('labels the three steps in lifecycle order', () => {
    expect(editorSteps().map((step) => step.label)).toEqual(['글 생성', '글 다듬기', '글 완성'])
  })
})
