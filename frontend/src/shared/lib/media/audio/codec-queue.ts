/** Healthy codecs wait for dequeue; only final/error boundaries drain them. */
export async function waitAudioCodecCapacity(
  codec: EventTarget & { encodeQueueSize?: number; decodeQueueSize?: number },
  maximum: number,
  signal: AbortSignal,
  timeoutMs: number,
) {
  signal.throwIfAborted()
  const size = () => codec.encodeQueueSize ?? codec.decodeQueueSize ?? Infinity
  if (size() < maximum) return
  await new Promise<void>((resolve, reject) => {
    const finish = (error?: unknown) => {
      clearTimeout(timer)
      codec.removeEventListener('dequeue', check)
      signal.removeEventListener('abort', abort)
      if (error) reject(error)
      else resolve()
    }
    const check = () => {
      if (size() < maximum) finish()
    }
    const abort = () => finish(signal.reason)
    const timer = setTimeout(() => finish(new Error('AUDIO_CODEC_TIMEOUT')), timeoutMs)
    codec.addEventListener('dequeue', check)
    signal.addEventListener('abort', abort, { once: true })
    check()
    if (signal.aborted) abort()
  })
}
export async function drainAudioCodec(
  codec: { flush(): Promise<void> },
  signal: AbortSignal,
  timeoutMs: number,
) {
  signal.throwIfAborted()
  await new Promise<void>((resolve, reject) => {
    const finish = (error?: unknown) => {
      clearTimeout(timer)
      signal.removeEventListener('abort', abort)
      if (error) reject(error)
      else resolve()
    }
    const abort = () => finish(signal.reason)
    const timer = setTimeout(() => finish(new Error('AUDIO_CODEC_TIMEOUT')), timeoutMs)
    signal.addEventListener('abort', abort, { once: true })
    void codec.flush().then(() => finish(), finish)
    if (signal.aborted) abort()
  })
}
