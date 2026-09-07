const loads = new Map<string, Promise<void>>()

/** Loads one third-party browser script at most once per page, sharing the in-flight result. */
export function loadScript(src: string): Promise<void> {
  const pending = loads.get(src)
  if (pending) return pending

  const promise = new Promise<void>((resolve, reject) => {
    const existing = Array.from(document.scripts).find((script) => script.src === src)
    const script = existing ?? document.createElement('script')
    const onLoad = () => {
      cleanup()
      resolve()
    }
    const onError = () => {
      cleanup()
      loads.delete(src)
      if (!existing) script.remove()
      reject(new Error(`Could not load script: ${src}`))
    }
    const cleanup = () => {
      script.removeEventListener('load', onLoad)
      script.removeEventListener('error', onError)
    }

    script.addEventListener('load', onLoad, { once: true })
    script.addEventListener('error', onError, { once: true })
    if (!existing) {
      script.async = true
      script.src = src
      document.head.append(script)
    }
  })
  loads.set(src, promise)
  return promise
}
