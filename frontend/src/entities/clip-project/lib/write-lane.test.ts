import { expect, it } from 'vitest'
import { serialClipWrite } from './write-lane'

// CLIP-188: a project's writes go one at a time, each after the one before has answered, and a
// failed write neither stops the lane nor reaches the next write's caller.
it('runs one project’s writes in order, one at a time, past a failure', async () => {
  const order: string[] = []
  let release!: () => void
  const held = new Promise<void>((resolve) => {
    release = resolve
  })
  const first = serialClipWrite('clip', async () => {
    order.push('first:start')
    await held
    order.push('first:end')
    throw new Error('refused')
  })
  const second = serialClipWrite('clip', async () => {
    order.push('second')
    return 2
  })
  const other = serialClipWrite('other', async () => {
    order.push('other')
    return 3
  })
  await expect(other).resolves.toBe(3)
  expect(order).toEqual(['first:start', 'other'])
  release()
  await expect(first).rejects.toThrow('refused')
  await expect(second).resolves.toBe(2)
  expect(order).toEqual(['first:start', 'other', 'first:end', 'second'])
})
