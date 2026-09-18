import { normalizeLoudness48k } from './loudness'
import { stretchStereo } from './stretch'
import { verifyEncodedAudio } from './verify-encoded'
import type { AudioWorkerRequest, AudioWorkerResponse, EncodedAudioTrack } from './processing-types'

const send = (message: AudioWorkerResponse, transfer: Transferable[] = []) =>
  self.postMessage(message, { transfer })
async function encode(
  request: Extract<AudioWorkerRequest, { kind: 'encode' }>,
): Promise<EncodedAudioTrack> {
  const { config, channels } = request
  const sampleFrames = channels[0].length
  const chunks: EncodedAudioTrack['chunks'] = []
  let decoderConfig: AudioDecoderConfig | undefined, failure: DOMException | undefined
  let onFailure: ((error: DOMException) => void) | undefined
  const encoder = new AudioEncoder({
    output: (chunk, metadata) => {
      const data = new Uint8Array(chunk.byteLength)
      chunk.copyTo(data)
      chunks.push({
        data,
        type: chunk.type,
        timestamp: chunk.timestamp,
        duration: chunk.duration ?? 0,
      })
      if (metadata?.decoderConfig) decoderConfig = metadata.decoderConfig
    },
    error: (error) => {
      failure = error
      onFailure?.(error)
    },
  })
  try {
    encoder.configure(config)
    for (let start = 0; start < sampleFrames; start += request.batchFrames) {
      if (failure) throw failure
      if (encoder.encodeQueueSize >= request.queueSize)
        await new Promise<void>((resolve, reject) => {
          const cleanup = () => {
            encoder.removeEventListener('dequeue', dequeued)
            onFailure = undefined
          }
          const dequeued = () => {
            if (encoder.encodeQueueSize < request.queueSize) {
              cleanup()
              resolve()
            }
          }
          onFailure = (error) => {
            cleanup()
            reject(error)
          }
          encoder.addEventListener('dequeue', dequeued)
        })
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
    await encoder.flush()
    if (failure) throw failure
    if (!decoderConfig || !chunks.length) throw new Error('AUDIO_TRACK_INVALID')
    const measured = await verifyEncodedAudio(chunks, decoderConfig, channels)
    return {
      config,
      decoderConfig,
      chunks,
      sampleFrames,
      durationUs: Math.round((sampleFrames * 1_000_000) / config.sampleRate),
      ...measured,
    }
  } finally {
    if (encoder.state !== 'closed') encoder.close()
  }
}
async function process(request: AudioWorkerRequest) {
  if (request.kind === 'stretch') {
    const result = stretchStereo(
      request.channels,
      request.sampleRate,
      request.rate,
      request.frames,
      request.gain,
    )
    send(
      { id: request.id, kind: 'result', result },
      result.map((channel) => channel.buffer),
    )
  } else if (request.kind === 'normalize') {
    const result = {
      channels: request.channels,
      ...normalizeLoudness48k(request.channels, request.target, request.ceiling),
    }
    send(
      { id: request.id, kind: 'result', result },
      request.channels.map((channel) => channel.buffer),
    )
  } else {
    const result = await encode(request)
    send(
      { id: request.id, kind: 'result', result },
      result.chunks.map((chunk) => chunk.data.buffer),
    )
  }
}
self.onmessage = (event: MessageEvent<AudioWorkerRequest>) => {
  void process(event.data).catch((error: unknown) =>
    send({
      id: event.data.id,
      kind: 'error',
      error: error instanceof Error ? error.message : String(error),
    }),
  )
}
