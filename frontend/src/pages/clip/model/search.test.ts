import { describe, expect, it } from 'vitest'
import { clipCreationSearchSchema } from './search'

describe('clipCreationSearchSchema', () => {
  it('keeps a handed template and the setup handoff, and ignores an old query parameter', () => {
    expect(clipCreationSearchSchema({ channel: 'old', template: 'tpl-1', from: 'setup' })).toEqual({
      template: 'tpl-1',
      from: 'setup',
    })
  })

  it('drops what it does not know rather than refusing it', () => {
    expect(clipCreationSearchSchema({ template: '  ', from: 'home' })).toEqual({
      template: undefined,
      from: undefined,
    })
    expect(clipCreationSearchSchema({ template: 7 })).toMatchObject({ template: undefined })
  })
})
