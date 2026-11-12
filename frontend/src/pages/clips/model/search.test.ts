import { expect, it } from 'vitest'
import { searchSchema } from './search'

it.each(['draft', 'storyline_ready', 'refining', 'confirmed', 'completed'])(
  'keeps the %s history filter and channel on a deep link',
  (status) => {
    expect(searchSchema({ status, channel: 'instagram_clip', q: '이야기' })).toEqual({
      status,
      channel: 'instagram_clip',
      q: '이야기',
    })
  },
)
it('drops unsupported status/channel values without refusing the list', () => {
  expect(searchSchema({ status: 'rendered', channel: 'foreign' })).toEqual({
    status: undefined,
    channel: undefined,
    q: undefined,
  })
})
it('opens a link saved with the former finished filter on 완료', () => {
  expect(searchSchema({ status: 'finished' })).toEqual({
    status: 'completed',
    channel: undefined,
    q: undefined,
  })
})
