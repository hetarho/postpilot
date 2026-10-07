import { Phases, sha256, shaJson } from './contract'

/** Isolated diagnostic hooks around the existing audio protocol. They neither
 * replace PCM, codecs, DSP, scheduling nor the production Worker implementation. */
export function observeProductionAudio() {
  const phases = new Phases()
  let active = true, nextWorker = 0, liveWorkers = 0, peakLiveWorkers = 0
  let createdAudioBufferBytes = 0, decodedAudioBufferBytes = 0, mixedAudioBufferBytes = 0
  let hashCopiedBytes = 0, peakHashCopiedBytes = 0
  const starts = new Map<string, { began: number; kind: string; bytes: number }>()
  const messages: { worker: number; id: number; kind: string; inputBytes: number; channels: number; sampleFrames: number; outputFrames?: number }[] = []
  const hashes: Promise<unknown>[] = [], normalizedPcm: unknown[] = []
  const workers: { value: Worker; message: (event: MessageEvent) => void; post: Worker['postMessage']; terminate: Worker['terminate'] }[] = []
  const originalWorker = window.Worker
  const createBuffer = BaseAudioContext.prototype.createBuffer
  const decodeAudioData = BaseAudioContext.prototype.decodeAudioData
  const startRendering = OfflineAudioContext.prototype.startRendering
  BaseAudioContext.prototype.createBuffer = new Proxy(createBuffer, {
    apply(target, receiver, args) {
      const buffer = Reflect.apply(target, receiver, args) as AudioBuffer
      if (active) createdAudioBufferBytes += buffer.length * buffer.numberOfChannels * 4
      return buffer
    },
  })
  BaseAudioContext.prototype.decodeAudioData = new Proxy(decodeAudioData, {
    apply(target, receiver, args) {
      const begin = performance.now(), result = Reflect.apply(target, receiver, args)
      if (result instanceof Promise) void result.then((buffer: AudioBuffer) => {
        if (active) { decodedAudioBufferBytes += buffer.length * buffer.numberOfChannels * 4; phases.add('speechDecodeAudioContext', performance.now() - begin) }
      }).catch(() => undefined)
      return result
    },
  })
  OfflineAudioContext.prototype.startRendering = new Proxy(startRendering, {
    apply(target, receiver, args) {
      const begin = performance.now(), result = Reflect.apply(target, receiver, args) as Promise<AudioBuffer>
      void result.then(buffer => {
        if (active) { mixedAudioBufferBytes = Math.max(mixedAudioBufferBytes, buffer.length * buffer.numberOfChannels * 4); phases.add('offlineMix', performance.now() - begin) }
      }).catch(() => undefined)
      return result
    },
  })
  window.Worker = new Proxy(originalWorker, {
    construct(target, args) {
      const value = Reflect.construct(target, args) as Worker
      const url = String(args[0])
      if (!/\/shared\/lib\/media\/audio\/processing\.worker\.ts(?:\?|$)/u.test(url)) return value
      const ordinal = ++nextWorker, post = value.postMessage, terminate = value.terminate
      liveWorkers++; peakLiveWorkers = Math.max(peakLiveWorkers, liveWorkers)
      let terminated = false
      const message = (event: MessageEvent) => {
        const data = event.data
        const key = `${ordinal}/${data.id}`
        const own = starts.get(key)
        if (!own || !active || !['result', 'error'].includes(data.kind)) return
        starts.delete(key)
        phases.add(`worker-${own.kind}`, performance.now() - own.began)
      }
      value.addEventListener('message', message)
      value.postMessage = new Proxy(post, {
        apply(method, receiver, arguments_) {
          const data = arguments_[0]
          if (active && ['stretch', 'normalize', 'encode'].includes(data?.kind) && Array.isArray(data.channels)) {
            const channels = data.channels as Float32Array<ArrayBuffer>[]
            if (!channels.length || channels.some(channel => !(channel instanceof Float32Array))) throw Error('BENCHMARK_AUDIO_PROTOCOL_CHANGED')
            const bytes = channels.reduce((n, channel) => n + channel.byteLength, 0)
            messages.push({ worker: ordinal, id: data.id, kind: data.kind, inputBytes: bytes,
              channels: channels.length, sampleFrames: channels[0].length, outputFrames: data.frames })
            starts.set(`${ordinal}/${data.id}`, { began: performance.now(), kind: data.kind, bytes })
            if (data.kind === 'encode') {
              // WebCrypto copies each BufferSource synchronously before the
              // actual postMessage transfers it. No diagnostic PCM reference
              // survives the production transfer, and no PCM is replaced.
              hashCopiedBytes += bytes; peakHashCopiedBytes = Math.max(peakHashCopiedBytes, hashCopiedBytes)
              const frames = channels[0].length, channelCount = channels.length, rate = data.config.sampleRate
              hashes.push((async () => {
                try {
                  const channelSha256 = await Promise.all(channels.map(channel => sha256(new Uint8Array(channel.buffer, channel.byteOffset, channel.byteLength))))
                  normalizedPcm.push({ frames, channels: channelCount, sampleRate: rate, bytes,
                    channelSha256, sha256: await shaJson({ frames, channels: channelCount, sampleRate: rate, channelSha256 }) })
                } finally { hashCopiedBytes -= bytes }
              })())
            }
          }
          return Reflect.apply(method, receiver, arguments_)
        },
      })
      value.terminate = new Proxy(terminate, {
        apply(method, receiver, arguments_) {
          if (!terminated) { terminated = true; liveWorkers-- }
          return Reflect.apply(method, receiver, arguments_)
        },
      })
      workers.push({ value, message, post, terminate })
      return value
    },
  })
  const restore = () => {
    active = false
    window.Worker = originalWorker
    BaseAudioContext.prototype.createBuffer = createBuffer
    BaseAudioContext.prototype.decodeAudioData = decodeAudioData
    OfflineAudioContext.prototype.startRendering = startRendering
    for (const worker of workers) {
      worker.value.removeEventListener('message', worker.message)
      worker.value.postMessage = worker.post; worker.value.terminate = worker.terminate
    }
  }
  return {
    restore,
    async finish() {
      try {
        await hashes.reduce((before, value) => before.then(() => value), Promise.resolve())
        const deadline = performance.now() + 1500
        while (liveWorkers && performance.now() < deadline) await new Promise(resolve => setTimeout(resolve, 10))
        return {
          phases: phases.snapshot(), messages, normalizedPcm,
          observedCreatedAudioBufferBytes: createdAudioBufferBytes,
          observedDecodedSpeechBufferBytes: decodedAudioBufferBytes,
          observedOfflineMixBufferBytes: mixedAudioBufferBytes,
          observedLargestPcmIpcBytes: Math.max(0, ...messages.map(message => message.inputBytes)),
          diagnosticHashCopiedPeakBytes: peakHashCopiedBytes,
          workerCount: workers.length, peakLiveWorkers, liveWorkersAfterProductionClose: liveWorkers,
          unresolvedRequests: starts.size,
          physicalPeakBytes: null, codecDspPrivatePeakBytes: null,
          scope: 'Actual created buffer sizes and protocol transfer sizes, not a measured full-heap peak; exact normalized pre-encode stereo PCM hash observes the entire selected mix',
          qualification: false,
        }
      } finally { restore() }
    },
  }
}
