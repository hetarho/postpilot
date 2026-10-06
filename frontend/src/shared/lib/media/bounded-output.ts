/** Position writes from a muxer. Overwrites are intentional, never append operations. */
export interface MediaPositionWrite {
  type: 'write'
  data: Uint8Array<ArrayBuffer>
  position: number
}
export interface BoundedMediaOutput {
  stream: WritableStream<MediaPositionWrite>
  kind: 'opfs' | 'blob'
  file(): Promise<Blob>
  dispose(): Promise<void>
  measurements(): { bytes: number; allocatedBytes: number; writes: number }
}
const lockName = (namespace: string, name: string) => `${namespace}/${name}`

/** A live file's Web Lock prevents another page's recovery from deleting it. */
export async function reclaimMediaOutputs(namespace: string): Promise<void> {
  if (!navigator.storage?.getDirectory || !navigator.locks) return
  const root = await navigator.storage.getDirectory()
  let directory: FileSystemDirectoryHandle
  try {
    directory = await root.getDirectoryHandle(namespace)
  } catch (error) {
    if (error instanceof DOMException && error.name === 'NotFoundError') return
    throw error
  }
  for await (const [name, handle] of directory.entries()) {
    if (handle.kind !== 'file' || !/^[a-zA-Z0-9_-]+\.mp4$/u.test(name)) continue
    await navigator.locks.request(
      lockName(namespace, name),
      { ifAvailable: true },
      async (lock) => {
        if (lock) await directory.removeEntry(name)
      },
    )
  }
}

/** The caller supplies finite budgets and an opaque run identity, never media or credentials. */
export async function createBoundedMediaOutput(
  options: {
    maxBytes: number
    pageBytes: number
    temporary?: { namespace: string; identity: string }
  },
  signal: AbortSignal,
): Promise<BoundedMediaOutput> {
  if (
    !Number.isSafeInteger(options.maxBytes) ||
    options.maxBytes <= 0 ||
    !Number.isSafeInteger(options.pageBytes) ||
    options.pageBytes <= 0 ||
    options.pageBytes > options.maxBytes
  )
    throw new Error('MEDIA_OUTPUT_BUDGET_INVALID')
  signal.throwIfAborted()
  let bytes = 0,
    writes = 0,
    stopped = false,
    closed = false
  let failure: unknown
  let finalizedFile: Blob | undefined
  const pages = new Map<number, Uint8Array<ArrayBuffer>>()
  let writer: FileSystemWritableFileStream | undefined
  let handle: FileSystemFileHandle | undefined
  let directory: FileSystemDirectoryHandle | undefined
  let name: string | undefined
  let release: (() => void) | undefined
  const temporary = options.temporary
  if (temporary && navigator.storage?.getDirectory && navigator.locks) {
    if (
      !/^[a-zA-Z0-9_-]{1,128}$/u.test(temporary.namespace) ||
      !/^[a-zA-Z0-9_-]{1,128}$/u.test(temporary.identity)
    )
      throw new Error('MEDIA_OUTPUT_IDENTITY_INVALID')
    await reclaimMediaOutputs(temporary.namespace)
    signal.throwIfAborted()
    directory = await (
      await navigator.storage.getDirectory()
    ).getDirectoryHandle(temporary.namespace, { create: true })
    name = `${temporary.identity}.mp4`
    const held = new Promise<void>((resolve) => {
      release = resolve
    })
    await new Promise<void>((resolve, reject) => {
      void navigator.locks
        .request(lockName(temporary.namespace, name!), { ifAvailable: true }, async (lock) => {
          if (!lock) {
            reject(new Error('MEDIA_OUTPUT_RUN_CONFLICT'))
            return
          }
          resolve()
          await held
        })
        .catch(reject)
    })
    try {
      handle = await directory.getFileHandle(name, { create: true })
      writer = await handle.createWritable()
    } catch (error) {
      await directory.removeEntry(name).catch(() => undefined)
      release?.()
      throw error
    }
  }
  let cleanup: Promise<void> | undefined
  const dispose = () => {
    cleanup ??= (async () => {
      stopped = true
      signal.removeEventListener('abort', abort)
      pages.clear()
      finalizedFile = undefined
      await writer?.abort().catch(() => undefined)
      if (directory && name) await directory.removeEntry(name).catch(() => undefined)
      release?.()
    })()
    return cleanup
  }
  const abort = () => {
    void dispose()
  }
  signal.addEventListener('abort', abort, { once: true })
  if (signal.aborted) {
    await dispose()
    signal.throwIfAborted()
  }
  const stream = new WritableStream<MediaPositionWrite>(
    {
      async write(chunk) {
        signal.throwIfAborted()
        if (stopped || closed) throw new Error('MEDIA_OUTPUT_CLOSED')
        const end = chunk.position + chunk.data.byteLength
        if (
          chunk.type !== 'write' ||
          !Number.isSafeInteger(chunk.position) ||
          chunk.position < 0 ||
          !Number.isSafeInteger(end) ||
          end > options.maxBytes
        ) {
          failure = new Error('MEDIA_OUTPUT_SIZE_LIMIT')
          throw failure
        }
        try {
          if (writer) await writer.write(chunk)
          else {
            let offset = 0
            while (offset < chunk.data.byteLength) {
              const position = chunk.position + offset
              const index = Math.floor(position / options.pageBytes)
              let page = pages.get(index)
              if (!page) {
                page = new Uint8Array(
                  Math.min(options.pageBytes, options.maxBytes - index * options.pageBytes),
                )
                pages.set(index, page)
              }
              const within = position % options.pageBytes
              const size = Math.min(page.byteLength - within, chunk.data.byteLength - offset)
              page.set(chunk.data.subarray(offset, offset + size), within)
              offset += size
            }
          }
          signal.throwIfAborted()
          bytes = Math.max(bytes, end)
          writes++
        } catch (error) {
          failure = error
          throw error
        }
      },
      async close() {
        signal.throwIfAborted()
        if (failure || stopped) throw failure ?? new Error('MEDIA_OUTPUT_CLOSED')
        try {
          await writer?.close()
          closed = true
        } catch (error) {
          failure = error
          throw error
        }
      },
      async abort(reason) {
        failure = reason
        await dispose()
      },
    },
    { highWaterMark: 1 },
  )
  return {
    stream,
    kind: writer ? 'opfs' : 'blob',
    dispose,
    async file() {
      signal.throwIfAborted()
      if (!closed || stopped || failure) throw failure ?? new Error('MEDIA_OUTPUT_INCOMPLETE')
      if (finalizedFile) return finalizedFile
      if (handle) {
        finalizedFile = await handle.getFile()
        return finalizedFile
      }
      const parts: BlobPart[] = []
      for (let index = 0; index * options.pageBytes < bytes; index++) {
        const length = Math.min(options.pageBytes, bytes - index * options.pageBytes)
        parts.push((pages.get(index) ?? new Uint8Array(length)).subarray(0, length))
      }
      const file = new Blob(parts, { type: 'video/mp4' })
      finalizedFile = file
      pages.clear()
      return file
    },
    measurements: () => ({
      bytes,
      writes,
      allocatedBytes: [...pages.values()].reduce((n, p) => n + p.byteLength, 0),
    }),
  }
}
