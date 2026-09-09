export class DirectUploadError extends Error {
  constructor(readonly status?: number) {
    super('Direct upload failed')
    this.name = 'DirectUploadError'
  }
}

/** Bytes go straight to storage. URLs and headers are never logged or cached. */
export function putBlobWithProgress(
  url: string,
  headers: Readonly<Record<string, string>>,
  blob: Blob,
  onProgress: (percent: number) => void,
  signal?: AbortSignal,
): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException('Upload aborted', 'AbortError'))
      return
    }
    const request = new XMLHttpRequest()
    let settled = false
    const finish = (error?: Error) => {
      if (settled) return
      settled = true
      signal?.removeEventListener('abort', abort)
      if (error) reject(error)
      else {
        onProgress(100)
        resolve()
      }
    }
    const abort = () => request.abort()
    request.open('PUT', url)
    request.withCredentials = false
    for (const [name, value] of Object.entries(headers)) request.setRequestHeader(name, value)
    request.upload.addEventListener('progress', (event) => {
      if (!settled && event.lengthComputable && event.total > 0)
        onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)))
    })
    request.addEventListener('load', () =>
      finish(
        request.status >= 200 && request.status < 300
          ? undefined
          : new DirectUploadError(request.status),
      ),
    )
    request.addEventListener('error', () => finish(new DirectUploadError()))
    request.addEventListener('timeout', () => finish(new DirectUploadError()))
    request.addEventListener('abort', () =>
      finish(new DOMException('Upload aborted', 'AbortError')),
    )
    signal?.addEventListener('abort', abort, { once: true })
    try {
      request.send(blob)
    } catch {
      finish(new DirectUploadError())
    }
  })
}
