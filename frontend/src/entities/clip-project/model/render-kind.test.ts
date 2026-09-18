import { expect, it } from 'vitest'
import { preferredClipRenderKind } from './render-kind'

it.each([
  [undefined, true, 'browser'],
  ['browser', true, 'browser'],
  ['server', true, 'server'],
  [undefined, false, 'server'],
  ['browser', false, 'server'],
  ['server', false, 'server'],
] as const)('prefers %s with browser availability %s as %s', (last, available, expected) => {
  expect(preferredClipRenderKind(last, available)).toBe(expected)
})
