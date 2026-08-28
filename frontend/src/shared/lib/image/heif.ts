// The main-thread side of the HEIC decoder: owns the worker, matches replies to requests.
//
// Imported dynamically by `decode.ts` — this module and the worker chunk it references
// must stay out of the main bundle, since most sessions never select a HEIC.
import { HEIF_DECODER_IDLE_MS } from '@/shared/config'
import { DecodeError } from './decode-error'
import type { HeifDecodeRequest, HeifDecodeResponse } from './heif-worker'

type Waiter = { resolve: (response: HeifDecodeResponse) => void; reject: (error: unknown) => void }

let worker: Worker | undefined
let idleTimer: number | undefined
const waiters = new Map<number, Waiter>()
let sequence = 0

function getWorker(): Worker {
  if (worker) return worker
  try {
    // Vite bundles the referenced module as its own chunk and only fetches it here.
    worker = new Worker(new URL('./heif-worker.ts', import.meta.url), { type: 'module' })
  } catch (error) {
    throw new DecodeError('heif-unsupported', error instanceof Error ? error.message : undefined)
  }
  worker.onmessage = (event: MessageEvent<HeifDecodeResponse>) => {
    const waiter = waiters.get(event.data.id)
    waiters.delete(event.data.id)
    waiter?.resolve(event.data)
    scheduleIdleTeardown()
  }
  // Only a worker that fails to come up ends here (the script or the WASM refused to
  // load — this device cannot decode HEIC); a bad file is answered, not thrown. It takes
  // every request in flight with it; the next file tries a fresh worker.
  worker.onerror = (event) => {
    const error = new DecodeError('heif-unsupported', event.message)
    for (const waiter of waiters.values()) waiter.reject(error)
    waiters.clear()
    teardown()
  }
  return worker
}

function teardown(): void {
  window.clearTimeout(idleTimer)
  idleTimer = undefined
  worker?.terminate()
  worker = undefined
}

function scheduleIdleTeardown(): void {
  window.clearTimeout(idleTimer)
  if (waiters.size > 0) return
  idleTimer = window.setTimeout(teardown, HEIF_DECODER_IDLE_MS)
}

/** Decodes a HEIC/HEIF file to a bitmap, or throws a `DecodeError`. */
export async function decodeHeif(file: Blob): Promise<ImageBitmap> {
  const id = ++sequence
  const response = await new Promise<HeifDecodeResponse>((resolve, reject) => {
    waiters.set(id, { resolve, reject })
    try {
      window.clearTimeout(idleTimer)
      const request: HeifDecodeRequest = { id, file }
      getWorker().postMessage(request)
    } catch (error) {
      waiters.delete(id)
      reject(error)
    }
  })
  if (!response.ok) throw new DecodeError('unreadable')

  const pixels = new ImageData(new Uint8ClampedArray(response.rgba), response.width, response.height)
  return createImageBitmap(pixels)
}
