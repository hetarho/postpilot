import { normalizeLoudness48k, loudnessRange48k } from './loudness'
import { stretchStereo } from './stretch'
import { verifyEncodedAudio } from './verify-encoded'
import { waitAudioCodecCapacity, drainAudioCodec } from './codec-queue'
import type { AudioWorkerRequest, AudioWorkerResponse, EncodedAudioTrack } from './processing-types'

const controller = new AbortController()
let active: Promise<unknown> | undefined
const send = (message: AudioWorkerResponse, transfer: Transferable[] = []) =>
  self.postMessage(message, { transfer })
async function encode(
  request: Extract<AudioWorkerRequest, { kind: 'encode' }>,
): Promise<EncodedAudioTrack> {
  const { config, channels, limits } = request
  const signal = controller.signal
  const sampleFrames = channels[0].length
  const chunks: EncodedAudioTrack['chunks'] = []
  let decoderConfig: AudioDecoderConfig | undefined,
    encodedBytes = 0
  const encoder = new AudioEncoder({
    output: (chunk, metadata) => {
      if (signal.aborted) return
      if (
        chunk.byteLength > limits.maxPacketBytes ||
        chunks.length >= limits.maxEncodedPackets ||
        encodedBytes + chunk.byteLength > limits.maxEncodedBytes
      ) {
        controller.abort(new Error('AUDIO_PACKET_MEMORY_LIMIT'))
        return
      }
      const data = new Uint8Array(chunk.byteLength)
      chunk.copyTo(data)
      chunks.push({
        data,
        type: chunk.type,
        timestamp: chunk.timestamp,
        duration: chunk.duration ?? 0,
      })
      encodedBytes += chunk.byteLength
      if (metadata?.decoderConfig) decoderConfig = metadata.decoderConfig
    },
    error: (error) => controller.abort(error),
  })
  const abort = () => {
    if (encoder.state !== 'closed') encoder.close()
  }
  signal.addEventListener('abort', abort, { once: true })
  try {
    signal.throwIfAborted()
    encoder.configure(config)
    for (let start = 0; start < sampleFrames; start += request.batchFrames) {
      await waitAudioCodecCapacity(encoder, request.queueSize, signal, limits.operationTimeoutMs)
      const count = Math.min(request.batchFrames, sampleFrames - start)
      const data = new Float32Array(count * channels.length)
      channels.forEach((channel, index) =>
        data.set(channel.subarray(start, start + count), index * count),
      )
      const audio = new AudioData({
        format: 'f32-planar',
        sampleRate: config.sampleRate,
        numberOfFrames: count,
        numberOfChannels: channels.length,
        timestamp: Math.round((start * 1_000_000) / config.sampleRate),
        data,
      })
      try {
        encoder.encode(audio)
      } finally {
        audio.close()
      }
      send({
        id: request.id,
        kind: 'progress',
        completedFrames: start + count,
        totalFrames: sampleFrames,
      })
    }
    await drainAudioCodec(encoder, signal, limits.operationTimeoutMs)
    if (!decoderConfig || !chunks.length) throw new Error('AUDIO_TRACK_INVALID')
    const measured = await verifyEncodedAudio(
      chunks,
      decoderConfig,
      channels,
      limits,
      signal,
      request.queueSize,
    )
    return {
      config,
      decoderConfig,
      chunks,
      sampleFrames,
      durationUs: Math.round((sampleFrames * 1_000_000) / config.sampleRate),
      ...measured,
    }
  } finally {
    signal.removeEventListener('abort', abort)
    abort()
  }
}
async function process(request: AudioWorkerRequest) {
  controller.signal.throwIfAborted()
  const inputBytes = request.channels.reduce((total, channel) => total + channel.byteLength, 0)
  const outputBytes =
    request.kind === 'stretch'
      ? request.frames * request.channels.length * Float32Array.BYTES_PER_ELEMENT
      : 0
  if (
    !Number.isSafeInteger(inputBytes + outputBytes) ||
    inputBytes + outputBytes > request.limits.maxPcmBytes ||
    !request.channels.length ||
    request.channels.some((channel) => channel.length !== request.channels[0].length)
  )
    throw new Error('AUDIO_PCM_MEMORY_LIMIT')
  if (request.kind === 'stretch') {
    const result = stretchStereo(
      request.channels,
      request.sampleRate,
      request.rate,
      request.frames,
      request.gain,
    )
    controller.signal.throwIfAborted()
    send(
      { id: request.id, kind: 'result', result },
      result.map((channel) => channel.buffer),
    )
  } else if (request.kind === 'normalize') {
    const loudnessRangeLU = loudnessRange48k(request.channels)
    if (!Number.isFinite(loudnessRangeLU) || loudnessRangeLU > request.rangeCeiling)
      throw new Error('AUDIO_LOUDNESS_RANGE_UNSUPPORTED')
    const result = {
      channels: request.channels,
      loudnessRangeLU,
      ...normalizeLoudness48k(request.channels, request.target, request.ceiling),
    }
    controller.signal.throwIfAborted()
    send(
      { id: request.id, kind: 'result', result },
      request.channels.map((channel) => channel.buffer),
    )
  } else {
    const result = await encode(request)
    controller.signal.throwIfAborted()
    send(
      { id: request.id, kind: 'result', result },
      result.chunks.map((chunk) => chunk.data.buffer),
    )
  }
}
self.onmessage = (event: MessageEvent<AudioWorkerRequest | { kind: 'cancel' }>) => {
  const request = event.data
  if (request.kind === 'cancel') {
    controller.abort(new DOMException('Audio processing cancelled', 'AbortError'))
    void Promise.resolve(active)
      .finally(() => send({ kind: 'cancelled' }))
      .catch(() => {})
    return
  }
  if (active) {
    send({ id: request.id, kind: 'error', error: 'AUDIO_WORKER_QUEUE_LIMIT' })
    return
  }
  const operation = process(request)
  active = operation
  void operation
    .catch((error: unknown) =>
      send({
        id: request.id,
        kind: 'error',
        error: error instanceof Error ? error.message : String(error),
      }),
    )
    .finally(() => {
      active = undefined
    })
}
