import { expect, it } from 'vitest'
import { navigationMenuTransition } from './navigation-menu'
it('guards duplicate open/close and closes every navigation', () => {
  expect(navigationMenuTransition('closed', 'close')).toBe('closed')
  expect(navigationMenuTransition('open', 'open')).toBe('open')
  expect(navigationMenuTransition('open', 'navigate')).toBe('closed')
  expect(navigationMenuTransition('closed', 'navigate')).toBe('closed')
  expect(navigationMenuTransition('closed', 'toggle')).toBe('open')
  expect(navigationMenuTransition('open', 'toggle')).toBe('closed')
})
