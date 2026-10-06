import {
  CLIP_AUDIO_PROCESSING,
  type BrowserCompositionSnapshot,
  type BrowserVideoTrack,
} from '@/entities/clip-preview'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design'
import { clipSourceSound } from '@/entities/clip-plan'
import {
  createBoundedMediaOutput,
  createMp4PacketMux,
  measureMp4Output,
  verifyMp4Audio,
} from '@/shared/lib/media'
import { BrowserOriginals } from '../lib/originals'
import { browserAudioPreflight } from '../model/audio-preflight'
import { browserRenderVerdict } from '../model/verdict'
import { BrowserRenderVerdictError, type VerifiedBrowserResult } from './store-result'
import type {
  BrowserRenderInput,
  BrowserRenderOperations,
  BrowserRenderProgress,
} from './run-render'

/** The artifact's lifetime belongs to the page; upload failure does not destroy it. */
export class BrowserUploadPendingError extends Error {
  constructor(
    readonly artifact: VerifiedBrowserResult,
    override readonly cause: unknown,
  ) {
    super('CLIP_UPLOAD_PENDING')
    this.name = 'BrowserUploadPendingError'
  }
}

export async function runLocalBrowserRender(
  input: BrowserRenderInput & { snapshot: BrowserCompositionSnapshot },
  operations: BrowserRenderOperations,
  signal: AbortSignal,
  progress: (value: BrowserRenderProgress) => void,
) {
  if (!('purpose' in input.snapshot) || input.snapshot.purpose !== 'export')
    throw new Error('CLIP_SNAPSHOT_EXPORT_PURPOSE_REQUIRED')
  if (
    input.snapshot.ownerId !== input.ownerId ||
    input.snapshot.projectId !== input.projectId ||
    input.snapshot.planRevision !== input.revision ||
    input.snapshot.ratio !== input.ratio
  )
    throw new Error('CLIP_SNAPSHOT_SUPERSEDED')
  if (!operations.bindSnapshot || !operations.storeLocal)
    throw new Error('CLIP_LOCAL_COMPOSITION_UNAVAILABLE')
  const controller = new AbortController()
  const abort = () => controller.abort(signal.reason)
  signal.addEventListener('abort', abort, { once: true })
  const originals = new BrowserOriginals(
    input.localSources,
    input.resolvePlayback,
    controller.signal,
  )
  let id: string | undefined
  let cancelRequest: Promise<boolean> | undefined
  const cancel = () => {
    if (id) {
      cancelRequest ??= operations.cancel(id)
      void cancelRequest.catch(() => undefined)
    }
    return cancelRequest
  }
  const cancelOnAbort = () => {
    void cancel()
  }
  signal.addEventListener('abort', cancelOnAbort, { once: true })
  let output: Awaited<ReturnType<typeof createBoundedMediaOutput>> | undefined
  let mux: Awaited<ReturnType<typeof createMp4PacketMux>> | undefined
  let video: ReturnType<BrowserRenderOperations['video']> | undefined
  let progressJob: Promise<void> | undefined
  let artifact: VerifiedBrowserResult | undefined
  let retained = false
  let audio: Awaited<ReturnType<BrowserRenderOperations['audio']>> = undefined
  try {
    signal.throwIfAborted()
    const sourceAudioCuts = input.plan.cuts.filter(
      (cut) =>
        clipSourceSound(input.plan, cut) &&
        input.snapshot.sources.some(
          (source) => source.sourceId === cut.sourceId && source.hasAudio,
        ),
    )
    const wantsAudio = !!input.plan.narration?.enabled || sourceAudioCuts.length > 0
    if (wantsAudio) browserAudioPreflight(input.plan)
    // Exact requested audio is checked before any video output can be delivered.
    audio = wantsAudio
      ? await operations.audio(
          input.plan,
          input.ratio,
          originals,
          controller.signal,
          (done, total) => progress({ stage: 'encoding', percent: (20 * done) / total }),
          input.loadSpeech,
        )
      : undefined
    if (input.plan.narration?.enabled && !audio) throw new Error('CLIP_SPEECH_UNAVAILABLE')
    if (
      sourceAudioCuts.length &&
      (!audio || (audio.sourceResources?.originals.length ?? 0) < sourceAudioCuts.length)
    )
      throw new Error('CLIP_SOURCE_AUDIO_UNAVAILABLE')
    const admitted = await operations.admit(input)
    id = admitted.renderId
    if (signal.aborted) {
      controller.abort(signal.reason)
      await cancel()
    }
    controller.signal.throwIfAborted()
    const snapshot = await operations.bindSnapshot!(input.snapshot, admitted)
    controller.signal.throwIfAborted()
    const identityBytes = new TextEncoder().encode(`${input.ownerId}/${input.projectId}/${id}`)
    const identity = [...new Uint8Array(await crypto.subtle.digest('SHA-256', identityBytes))]
      .map((v) => v.toString(16).padStart(2, '0'))
      .join('')
    output = await createBoundedMediaOutput(
      {
        maxBytes: CLIP_BROWSER_RENDER.temporaryOutputBytes,
        pageBytes: CLIP_BROWSER_RENDER.outputPageBytes,
        temporary: { namespace: CLIP_BROWSER_RENDER.outputNamespace, identity },
      },
      signal,
    )
    mux = await createMp4PacketMux(output, CLIP_BROWSER_RENDER.frameRate, audio, controller.signal)
    video = operations.video(
      { snapshot, plan: input.plan, ratio: input.ratio, assets: [] },
      input.localSources,
      input.resolvePlayback,
      controller.signal,
      originals,
      undefined,
      mux.video,
    )
    progressJob = (async () => {
      for await (const frame of video!.progress) {
        if (!controller.signal.aborted)
          progress({
            stage: 'encoding',
            percent: 20 + (60 * frame.completedFrames) / frame.totalFrames,
          })
      }
    })()
    void progressJob.catch((error) => controller.abort(error))
    const track: BrowserVideoTrack = await video.result
    await progressJob
    controller.signal.throwIfAborted()
    const file = await mux.finalize()
    track.outputMeasurements = await measureMp4Output(
      file,
      CLIP_BROWSER_RENDER.frameRate,
      snapshot.frameCount,
      controller.signal,
    )
    if (audio) {
      const measured = await verifyMp4Audio(
        file,
        {
          sampleFrames: audio.sampleFrames,
          sampleRate: audio.config.sampleRate,
          channels: audio.config.numberOfChannels,
          maxBytes: CLIP_AUDIO_PROCESSING.maxPcmBytes,
          paddingFrames: CLIP_AUDIO_PROCESSING.encoderPaddingFrames,
          maxSampleFrames: CLIP_AUDIO_PROCESSING.maxSampleFrames,
        },
        controller.signal,
      )
      audio.loudnessLUFS = measured.loudnessLUFS
      audio.truePeakDBTP = measured.truePeakDBTP
      audio.silent = measured.silent
    }
    const verdict = browserRenderVerdict(track, audio, input.ratio, input.plan.durationMs)
    if (!verdict.passed) throw new BrowserRenderVerdictError([])
    const ownedOutput = output
    artifact = { renderId: id, file, verdict, dispose: () => ownedOutput.dispose() }
    input.onLocalReady?.(artifact)
    controller.signal.throwIfAborted()
    progress({ stage: 'storing', percent: CLIP_BROWSER_RENDER.encodeProgressPercent })
    try {
      const project = await operations.storeLocal!(artifact, controller.signal, (percent) =>
        progress({ stage: 'storing', percent: 80 + (19 * percent) / 100 }),
      )
      controller.signal.throwIfAborted()
      await artifact.dispose()
      return project
    } catch (error) {
      if (signal.aborted || error instanceof BrowserRenderVerdictError) throw error
      retained = true
      throw new BrowserUploadPendingError(artifact, error)
    }
  } catch (error) {
    if (error instanceof BrowserUploadPendingError) throw error
    controller.abort(error)
    const cancelled = await cancel()?.catch(() => undefined)
    if (signal.aborted && cancelled === false) return operations.refresh(input.projectId)
    throw error
  } finally {
    controller.abort()
    video?.cancel()
    await Promise.allSettled([
      ...(video ? [video.result] : []),
      ...(progressJob ? [progressJob] : []),
    ])
    originals.dispose()
    if (audio) audio.chunks.length = 0
    if (!retained) {
      await mux?.cancel()
      await output?.dispose()
    }
    signal.removeEventListener('abort', abort)
    signal.removeEventListener('abort', cancelOnAbort)
  }
}
