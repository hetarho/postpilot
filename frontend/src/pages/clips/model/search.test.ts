import { expect, it } from 'vitest'
import { searchSchema } from './search'

it.each(['draft', 'storyline_ready', 'refining', 'confirmed', 'completed'])(
  'keeps the %s history filter on a deep link',
  (status) => {
    expect(searchSchema({ status, channel: 'old', q: '이야기' })).toEqual({
      status,
      q: '이야기',
    })
  },
)
it('drops unsupported status values without refusing the list', () => {
  expect(searchSchema({ status: 'rendered' })).toEqual({
    status: undefined,
    q: undefined,
  })
})
it('opens a link saved with the former finished filter on 완료', () => {
  expect(searchSchema({ status: 'finished' })).toEqual({
    status: 'completed',
    q: undefined,
  })
})
