import { canonicalSelectedAudio } from '@/features/render-clip-browser/lib/selected-audio'
import { renderBrowserAudio } from '@/features/render-clip-browser/api/render-audio'
import { clipTimelineFixture } from '@/test/clip-editing'
import {
  browserAudioPlan,
  CLIP_AUDIO_PROCESSING,
  clipBrowserEncoderConfig,
} from '@/entities/clip-preview'
import { createAudioProcessor, type SelectedAudioRange } from '@/shared/lib'

declare global {
  interface Window {
    audioRangeFixture: {
      codecRefusal(kind: 'packet' | 'verification'): Promise<unknown>
      decode(request: {
        id: string
        url: string
        startUs: number
        endUs: number
        cancel?: boolean
        memoryBytes?: number
      }): Promise<unknown>
      render(request: {
        id: string
        url: string
        rate?: number
        narration?: boolean
        retain?: boolean
        cancel?: boolean
        hook?: boolean
        seam?: boolean
        wrongSpeechHash?: boolean
        sourceVolume?: number
        narrationVolume?: number
      }): Promise<unknown>
    }
  }
}
async function save(id: string, channels: Float32Array[]) {
  const interleaved = new Float32Array(channels[0].length * channels.length)
  for (let frame = 0; frame < channels[0].length; frame++)
    for (let channel = 0; channel < channels.length; channel++)
      interleaved[frame * channels.length + channel] = channels[channel][frame]
  await fetch(`/__audio-range__/output/${id}.f32`, { method: 'POST', body: interleaved })
}
window.audioRangeFixture = {
  async codecRefusal(kind) {
    const limits = CLIP_AUDIO_PROCESSING
    const processor = createAudioProcessor(new AbortController().signal, {
      maxPcmBytes: limits.maxPcmBytes,
      maxEncodedBytes: kind === 'packet' ? 1 : limits.encodedPacketBytes,
      maxEncodedPackets: limits.encodedPackets,
      maxPacketBytes: limits.encodedPacketMaxBytes,
      maxPrimingFrames: kind === 'verification' ? 0 : limits.encoderPaddingFrames,
      operationTimeoutMs: limits.operationTimeoutMs,
      cleanupTimeoutMs: limits.cleanupTimeoutMs,
    })
    const channels = [0, 1].map(() =>
      Float32Array.from(
        { length: 48000 },
        (_, n) => 0.1 * Math.sin((2 * Math.PI * 440 * n) / 48000),
      ),
    )
    try {
      await processor.encode(channels, clipBrowserEncoderConfig('vertical').audio, 2048, 4)
      return { error: '' }
    } catch (error) {
      return { error: error instanceof Error ? error.message : String(error) }
    } finally {
      processor.close()
    }
  },
  async decode(request) {
    const worker = new Worker(new URL('./audio-range.worker.ts', import.meta.url), {
      type: 'module',
    })
    try {
      const message = await new Promise<{
        result?: SelectedAudioRange
        error?: string
        resources: unknown
      }>((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error('Fixture timeout')), 30000)
        worker.onmessage = (event) => {
          clearTimeout(timer)
          resolve(event.data)
        }
        worker.onerror = (event) => {
          clearTimeout(timer)
          reject(new Error(event.message))
        }
        worker.postMessage(request)
      })
      if (!message.result) return message
      const { metadata, measurements, decodeStart, startSample } = message.result
      const channels = await canonicalSelectedAudio(
        message.result,
        Math.round((request.startUs * 48000) / 1_000_000),
        Math.round(((request.endUs - request.startUs) * 48000) / 1_000_000),
        48000,
        new AbortController().signal,
      )
      await save(request.id, channels)
      let currentReference: { rmse: number; maxDifference: number } | undefined
      if (metadata.sampleRate !== 48000) {
        const decoder = new AudioContext({ sampleRate: 48000 })
        try {
          const original = await decoder.decodeAudioData(
            await (await fetch(request.url.replace('/file/', '/reference/'))).arrayBuffer(),
          )
          const offset = Math.round((request.startUs * 48000) / 1_000_000)
          let sum = 0,
            max = 0
          for (let n = 0; n < channels[0].length; n++) {
            const difference =
              channels[0][n] - original.getChannelData(0)[offset + n] * Math.SQRT1_2
            sum += difference ** 2
            max = Math.max(max, Math.abs(difference))
          }
          currentReference = { rmse: Math.sqrt(sum / channels[0].length), maxDifference: max }
        } finally {
          await decoder.close()
        }
      }
      return {
        metadata,
        measurements,
        decodeStart,
        startSample,
        frames: channels[0].length,
        currentReference,
        resources: message.resources,
      }
    } finally {
      worker.terminate()
    }
  },
  async render(request) {
    const plan = clipTimelineFixture().plan
    const original = plan.cuts[0]
    plan.cuts = [
      {
        ...original,
        id: 'source',
        startMs: 0,
        endMs: 2000,
        playbackRatePermille: request.rate ?? 1000,
        transitionMs: 0,
      },
    ]
    if (request.seam)
      plan.cuts = [
        { ...plan.cuts[0], id: 'first', endMs: 1234 },
        { ...plan.cuts[0], id: 'second', startMs: 2000, endMs: 3234, playbackRatePermille: 750 },
      ]
    plan.durationMs = 2000
    plan.elements = request.hook
      ? [
          {
            ...plan.elements![0],
            role: 'hook',
            text: 'declared hook',
            rows: [],
            cutId: '',
            basis: 'output-start',
            startMs: 0,
            endMs: 1000,
          },
        ]
      : []
    plan.sourceAudio = [
      {
        sourceId: original.sourceId,
        fingerprint: original.fingerprint,
        retainOriginalAudio: request.retain ?? true,
      },
    ]
    plan.sourceVolumePermille = request.sourceVolume ?? 1000
    let speechCalls = 0,
      sourceCalls = 0
    const speechBytes = request.narration
      ? await (await fetch('/__audio-range__/reference/speech')).arrayBuffer()
      : undefined
    if (speechBytes) {
      const audioHash = [...new Uint8Array(await crypto.subtle.digest('SHA-256', speechBytes))]
        .map((v) => v.toString(16).padStart(2, '0'))
        .join('')
      plan.narration = {
        enabled: true,
        confirmedVoiceId: 'fixture',
        bindingDigest: 'fixture',
        volumePermille: 700,
        segments: [
          {
            id: 'spoken',
            text: 'synthetic tone',
            textRevision: 1,
            inputHash: 'script',
            startMs: 500,
            endMs: 1500,
            speech: {
              assetId: 'fixture',
              voiceId: 'fixture',
              bindingDigest: 'fixture',
              inputHash: 'script',
              settingsHash: 'fixture',
              audioHash,
              profileId: 'fixture',
              profileRevision: 1,
              samples: 44100,
              sampleRate: 44100,
              channels: 2,
              timing: [],
            },
          },
        ],
      }
      plan.narration.volumePermille = request.narrationVolume ?? 700
      if (request.wrongSpeechHash) plan.narration.segments[0].speech!.audioHash = '0'.repeat(64)
    } else plan.narration = undefined
    const schedule = browserAudioPlan(plan),
      controller = new AbortController()
    try {
      const result = await renderBrowserAudio(
        plan,
        'vertical',
        {
          source: async () => {
            sourceCalls++
            return { kind: 'url', url: request.url }
          },
        },
        controller.signal,
        () => {
          if (request.cancel) controller.abort(new DOMException('Fixture cancelled', 'AbortError'))
        },
        async () => {
          speechCalls++
          return speechBytes!.slice(0)
        },
      )
      if (!result) return { silent: true, sourceCalls, speechCalls }
      const decoder = new AudioDecoder({
        output: (data) => {
          try {
            const pcm = Array.from({ length: data.numberOfChannels }, (_, channel) => {
              const plane = new Float32Array(data.numberOfFrames)
              data.copyTo(plane, { format: 'f32-planar', planeIndex: channel })
              return plane
            })
            parts.push(pcm)
            decodedFrames += data.numberOfFrames
          } finally {
            data.close()
          }
        },
        error: (error) => {
          failure = error
        },
      })
      const parts: Float32Array[][] = []
      let decodedFrames = 0,
        failure: DOMException | undefined
      try {
        decoder.configure(result.decoderConfig)
        for (const chunk of result.chunks) decoder.decode(new EncodedAudioChunk(chunk))
        await decoder.flush()
        if (failure) throw failure
      } finally {
        decoder.close()
      }
      const full = [new Float32Array(decodedFrames), new Float32Array(decodedFrames)]
      let offset = 0
      for (const part of parts) {
        part.forEach((plane, channel) => full[channel].set(plane, offset))
        offset += part[0].length
      }
      const aligned = full.map((plane) =>
        plane.slice(result.primingFrames, result.primingFrames + result.sampleFrames),
      )
      await save(request.id, aligned)
      return {
        sampleFrames: result.sampleFrames,
        scheduleFrames: schedule.sampleFrames,
        durationUs: result.durationUs,
        primingFrames: result.primingFrames,
        loudnessLUFS: result.loudnessLUFS,
        truePeakDBTP: result.truePeakDBTP,
        sourceResources: result.sourceResources,
        speechFingerprint: result.speechFingerprint,
        sourceCalls,
        speechCalls,
        sourceWindows: schedule.cuts.map((cut) => ({
          startSample: cut.startSample,
          frames: cut.frames,
          fadeOutStart: cut.fadeOutStart,
        })),
      }
    } catch (error) {
      return {
        error: error instanceof Error ? error.message : String(error),
        name: error instanceof Error ? error.name : '',
        sourceCalls,
        speechCalls,
      }
    }
  },
}
