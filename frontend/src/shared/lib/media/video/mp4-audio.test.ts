import { Blob as NodeBlob } from 'node:buffer'
import { expect, it } from 'vitest'
import { mp4HasAudio } from './mp4-audio'

function box(tag: string, body: Uint8Array, extended = false) {
  const data = new Uint8Array(body.length + (extended ? 16 : 8))
  const view = new DataView(data.buffer)
  view.setUint32(0, extended ? 1 : data.length)
  data.set(new TextEncoder().encode(tag), 4)
  if (extended) view.setBigUint64(8, BigInt(data.length))
  data.set(body, extended ? 16 : 8)
  return data
}
function track(handler: string) {
  const data = new Uint8Array(12)
  data.set(new TextEncoder().encode(handler), 8)
  return box('trak', box('mdia', box('hdlr', data)))
}
it('finds a sound handler through extended-size movie boxes without decoding media payload', async () => {
  const video = track('vide'),
    audio = track('soun')
  const body = new Uint8Array(video.length + audio.length)
  body.set(video)
  body.set(audio, video.length)
  const blob = new NodeBlob([
    box('mdat', new Uint8Array(100000)),
    box('moov', body, true),
  ]) as unknown as Blob
  expect(await mp4HasAudio(blob, new AbortController().signal)).toBe(true)
})
it('distinguishes a video-only file from a malformed audio input', async () => {
  const silent = new NodeBlob([box('moov', track('vide'))]) as unknown as Blob
  expect(await mp4HasAudio(silent, new AbortController().signal)).toBe(false)
  const broken = new NodeBlob([new Uint8Array([0, 0, 0, 4, 109, 111, 111, 118])]) as unknown as Blob
  await expect(mp4HasAudio(broken, new AbortController().signal)).rejects.toThrow(
    'Invalid MP4 box size',
  )
})
