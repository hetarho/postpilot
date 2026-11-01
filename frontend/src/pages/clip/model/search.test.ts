import { describe, expect, it } from 'vitest'
import { clipCreationSearchSchema } from './search'

describe('clipCreationSearchSchema', () => {
  it('keeps a known channel, a handed template and the setup handoff', () => {
    expect(
      clipCreationSearchSchema({ channel: 'instagram_clip', template: 'tpl-1', from: 'setup' }),
    ).toEqual({ channel: 'instagram_clip', template: 'tpl-1', from: 'setup' })
  })

  it('drops what it does not know rather than refusing it', () => {
    expect(clipCreationSearchSchema({ channel: 'tiktok', template: '  ', from: 'home' })).toEqual({
      channel: undefined,
      template: undefined,
      from: undefined,
    })
    expect(clipCreationSearchSchema({ template: 7 })).toMatchObject({ template: undefined })
  })
})
