import { CustomSource } from 'mediabunny'

/** Page/worker-owned access. Never serialize this into a plan or durable snapshot. */
export type BrowserMediaSourceAccess = { kind: 'blob'; blob: Blob } | { kind: 'url'; url: string }
export interface MediaRangeLimits {
  maxFileBytes: number
  maxReadBytes: number
  maxReadTotalBytes: number
  maxCacheBytes: number
  timeoutMs: number
}
export interface MediaRangeMeasurements {
  reads: number
  bytesRead: number
  largestReadBytes: number
  sizeBytes: number
}
export class MediaRangeError extends Error {
  constructor(readonly code: string) {
    super(code)
  }
}
export interface MediaRangePorts {
  fetch?: typeof fetch
  read?: (bytes: number, elapsedMs: number) => void
}

/** A finite range reader with no network prefetch or eager full-original fetch.
 * https://mediabunny.dev/guide/reading-media-files#customsource
 */
export function createFiniteMediaSource(
  access: BrowserMediaSourceAccess,
  limits: MediaRangeLimits,
  signal: AbortSignal,
  ports: MediaRangePorts = {},
) {
  for (const n of Object.values(limits))
    if (!Number.isSafeInteger(n) || n <= 0) throw new MediaRangeError('CLIP_SOURCE_MEMORY_LIMIT')
  const controller = new AbortController()
  const abort = () => controller.abort(signal.reason)
  signal.addEventListener('abort', abort, { once: true })
  if (signal.aborted) abort()
  const state: MediaRangeMeasurements = {
    reads: 0,
    bytesRead: 0,
    largestReadBytes: 0,
    sizeBytes: 0,
  }
  let failure: unknown
  let sizePromise: Promise<number> | undefined
  const fetcher = ports.fetch ?? fetch
  const check = () => {
    controller.signal.throwIfAborted()
    if (failure) throw failure
  }
  function fail(code: string): never {
    throw new MediaRangeError(code)
  }
  const validateSize = (size: number) => {
    if (!Number.isSafeInteger(size) || size <= 0 || size > limits.maxFileBytes)
      fail('CLIP_SOURCE_MEMORY_LIMIT')
    state.sizeBytes = size
    return size
  }
  const request = async (start: number, end: number, expectedSize?: number) => {
    check()
    const deadline = setTimeout(
      () => controller.abort(new MediaRangeError('CLIP_SOURCE_TIMEOUT')),
      limits.timeoutMs,
    )
    let response: Response | undefined
    let readerOpened = false
    try {
      response = await fetcher((access as { kind: 'url'; url: string }).url, {
        headers: { Range: `bytes=${start}-${end - 1}` },
        signal: controller.signal,
      })
      if (response.status !== 206) {
        if (response.status === 401 || response.status === 403) fail('CLIP_SOURCE_EXPIRED')
        if (response.status === 404 || response.status === 410) fail('CLIP_SOURCE_MISSING')
        fail('CLIP_SOURCE_RANGE_UNSUPPORTED')
      }
      const match = response.headers.get('Content-Range')?.match(/^bytes (\d+)-(\d+)\/(\d+)$/u)
      const size = match ? Number(match[3]) : 0
      if (
        !match ||
        Number(match[1]) !== start ||
        Number(match[2]) !== end - 1 ||
        (expectedSize !== undefined && size !== expectedSize)
      ) {
        fail('CLIP_SOURCE_RANGE_INVALID')
      }
      validateSize(size)
      const length = response.headers.get('Content-Length')
      if (length !== null && Number(length) !== end - start) fail('CLIP_SOURCE_RANGE_INVALID')
      if (!response.body) fail('CLIP_SOURCE_RANGE_INVALID')
      // Do not trust Content-Length or arrayBuffer(): stop a lying/chunked
      // response before it can materialize an unbounded body.
      const bytes = new Uint8Array(end - start)
      const reader = response.body.getReader()
      readerOpened = true
      let offset = 0
      try {
        for (;;) {
          const result = await reader.read()
          check()
          if (result.done) break
          if (offset + result.value.byteLength > bytes.byteLength) fail('CLIP_SOURCE_RANGE_INVALID')
          bytes.set(result.value, offset)
          offset += result.value.byteLength
        }
        if (offset !== bytes.byteLength) fail('CLIP_SOURCE_RANGE_INVALID')
        return { size, bytes }
      } finally {
        await reader.cancel().catch(() => undefined)
        reader.releaseLock()
      }
    } finally {
      if (!readerOpened) await response?.body?.cancel().catch(() => undefined)
      clearTimeout(deadline)
    }
  }
  const getSize = () =>
    (sizePromise ??= (async () => {
      check()
      if (access.kind === 'blob') return validateSize(access.blob.size)
      // Owner-bound presigned GET does not authorize HEAD. One finite byte also
      // proves that this server honors ranges before reading media.
      return (await request(0, 1)).size
    })())
  const read = async (start: number, end: number) => {
    check()
    const size = await getSize()
    const count = end - start
    if (
      !Number.isSafeInteger(start) ||
      !Number.isSafeInteger(end) ||
      start < 0 ||
      end > size ||
      count <= 0 ||
      count > limits.maxReadBytes ||
      state.bytesRead + count > limits.maxReadTotalBytes
    )
      fail('CLIP_SOURCE_MEMORY_LIMIT')
    const began = performance.now()
    // Reserve accounting before await so concurrent parser requests share the cap.
    state.bytesRead += count
    state.reads++
    state.largestReadBytes = Math.max(state.largestReadBytes, count)
    const bytes =
      access.kind === 'blob'
        ? new Uint8Array(await access.blob.slice(start, end).arrayBuffer())
        : (await request(start, end, size)).bytes
    check()
    ports.read?.(count, performance.now() - began)
    return bytes
  }
  const source = new CustomSource({
    getSize,
    read,
    dispose() {
      signal.removeEventListener('abort', abort)
      controller.abort()
    },
    maxCacheSize: limits.maxCacheBytes,
    prefetchProfile: 'none',
    handleUnhandledError(error) {
      failure = error
      controller.abort(error)
    },
  })
  return {
    source,
    getSize,
    read,
    measurements: () => ({ ...state }),
    dispose: () => {
      signal.removeEventListener('abort', abort)
      controller.abort()
    },
  }
}
