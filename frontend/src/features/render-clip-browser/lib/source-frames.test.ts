import { expect, it, vi } from 'vitest'
import { BrowserSourceFrames } from './source-frames'

it('fetches an unheld original once before its first frame, and reuses it across cuts', async () => {
  const events: string[] = []
  const source = {
    frame: vi.fn(async (ms: number) => {
      events.push(`frame:${ms}`)
      return {} as ImageBitmap
    }),
    close: vi.fn(),
  }
  const ports = {
    read: vi.fn(async () => {
      events.push('fetch')
      return new Blob()
    }),
    open: vi.fn(async () => {
      events.push('open')
      return source
    }),
  }
  const frames = new BrowserSourceFrames(ports, new AbortController().signal)
  await frames.frame('fingerprint', 1000)
  await frames.frame('fingerprint', 5000)
  expect(events).toEqual(['fetch', 'open', 'frame:1000', 'frame:5000'])
  expect(ports.read).toHaveBeenCalledOnce()
  frames.dispose()
  await Promise.resolve()
  expect(source.close).toHaveBeenCalledOnce()
})

it('cancellation after a source read never opens its decoder', async () => {
  const controller = new AbortController()
  const ports = {
    read: vi.fn(async () => {
      controller.abort()
      return new Blob()
    }),
    open: vi.fn(),
  }
  const frames = new BrowserSourceFrames(ports, controller.signal)
  await expect(frames.frame('fp', 0)).rejects.toMatchObject({ name: 'AbortError' })
  expect(ports.read).toHaveBeenCalledOnce()
  expect(ports.open).not.toHaveBeenCalled()
  frames.dispose()
})
