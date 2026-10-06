import { expect, it, vi } from 'vitest'
import { MediaPacketWindow } from './packet-window'

it('waits for slow packet acceptance while reserving room for outstanding encoder callbacks', async () => {
  const window = new MediaPacketWindow(
    { packets: 4, bytes: 16, timeoutMs: 1000 },
    new AbortController().signal,
  )
  const ids: number[] = []
  for (let i = 0; i < 3; i++) window.send(4, (id) => ids.push(id))
  const ready = vi.fn()
  const waiting = window.capacity(1).then(ready)
  await Promise.resolve()
  expect(ready).not.toHaveBeenCalled()
  window.ack(ids[0]!)
  await waiting
  expect(window.measurements()).toEqual({ peakPackets: 3, peakBytes: 12 })
  const drain = window.drain()
  window.ack(ids[1]!)
  window.ack(ids[2]!)
  await drain
})
it('propagates delayed sink failure and refuses packets exceeding the byte bound', async () => {
  const window = new MediaPacketWindow(
    { packets: 2, bytes: 8, timeoutMs: 1000 },
    new AbortController().signal,
  )
  let id = 0
  window.send(8, (value) => {
    id = value
  })
  expect(() => window.send(1, () => {})).toThrow('MEDIA_PACKET_WINDOW_LIMIT')
  const drained = expect(window.drain()).rejects.toThrow('disk failed')
  window.ack(id, new Error('disk failed'))
  await drained
  await expect(window.capacity()).rejects.toThrow('disk failed')
})
it('cancels a waiting sink without flushing healthy encoder work', async () => {
  const controller = new AbortController()
  const window = new MediaPacketWindow({ packets: 1, bytes: 8, timeoutMs: 1000 }, controller.signal)
  window.send(4, () => {})
  const waiting = expect(window.drain()).rejects.toThrow('cancelled')
  controller.abort(new Error('cancelled'))
  await waiting
})
it('reserves all submitted frames until their asynchronous output has been accepted', async () => {
  const window = new MediaPacketWindow(
    { packets: 2, bytes: 8, timeoutMs: 1000 },
    new AbortController().signal,
  )
  await window.reserve()
  await window.reserve()
  const ready = vi.fn(),
    waiting = window.reserve().then(ready)
  await Promise.resolve()
  expect(ready).not.toHaveBeenCalled()
  let first = 0
  window.send(4, (id) => {
    first = id
  })
  await Promise.resolve()
  expect(ready).not.toHaveBeenCalled()
  window.ack(first)
  await waiting
})
