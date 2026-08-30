// HEIC/HEIF → RGBA, off the main thread.
//
// libheif's decode is synchronous CPU work — around a second for a 12 MP phone photo on a
// mid-range device — and eight of them back to back would freeze the editor for the
// duration (PRD §6.2 wants the conversion visible, not the page hung). The 1 MB WASM
// binary is embedded in this module's chunk, so it is only fetched when the first
// HEIC is selected (the worker is constructed lazily in `heif.ts`).
import createLibheif from 'libheif-js/libheif-wasm/libheif-bundle.mjs'

export interface HeifDecodeRequest {
  id: number
  /** The file itself, not its bytes: a Blob crosses to a worker by reference, so reading
   *  it here keeps the (up to 25 MB) original off the main thread entirely. */
  file: Blob
}

export type HeifDecodeResponse =
  | { id: number; ok: true; width: number; height: number; rgba: ArrayBuffer }
  | { id: number; ok: false }

const libheif = createLibheif()
const decoder = new libheif.HeifDecoder()

// One file at a time. `HeifDecoder.decode` frees the previous file's context, so a
// second decode arriving while the first's `display` is still pending would pull the
// image out from under it; the main thread does not have to know this.
let queue: Promise<void> = Promise.resolve()

self.onmessage = (event: MessageEvent<HeifDecodeRequest>) => {
  queue = queue.then(() => handle(event.data))
}

async function handle({ id, file }: HeifDecodeRequest): Promise<void> {
  try {
    const result = await decode(await file.arrayBuffer())
    if (!result) {
      postFailure(id)
      return
    }
    const response: HeifDecodeResponse = { id, ok: true, ...result }
    self.postMessage(response, { transfer: [result.rgba] })
  } catch {
    // A corrupt file makes the decoder throw; that is the file's fault, not the
    // device's, and it must not take the worker (and the other files queued) with it.
    postFailure(id)
  }
}

function decode(
  buffer: ArrayBuffer,
): Promise<{ width: number; height: number; rgba: ArrayBuffer } | null> {
  const images = decoder.decode(buffer)
  // A HEIC from a phone is one primary image plus, at times, auxiliary ones (depth map,
  // burst frames). The primary is the photo.
  const image = images.find((candidate) => candidate.is_primary()) ?? images[0]
  if (!image) return Promise.resolve(null)

  const width = image.get_width()
  const height = image.get_height()
  const target = { data: new Uint8ClampedArray(width * height * 4), width, height }
  return new Promise((resolve) => {
    image.display(target, (result) => {
      for (const each of images) each.free()
      resolve(result ? { width, height, rgba: target.data.buffer } : null)
    })
  })
}

function postFailure(id: number) {
  const response: HeifDecodeResponse = { id, ok: false }
  self.postMessage(response)
}
