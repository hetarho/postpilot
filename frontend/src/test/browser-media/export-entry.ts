import { freezeBrowserComposition, CLIP_AUDIO_PROCESSING } from '@/entities/clip-preview'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design'
import type { ClipRatio } from '@/entities/clip-project'
import type { ClipEditPlan } from '@/entities/clip-plan'
import { renderBrowserVideo } from '@/features/render-clip-browser/api/render-video'
import { renderBrowserAudio } from '@/features/render-clip-browser/api/render-audio'
import { browserRenderVerdict } from '@/features/render-clip-browser/model/verdict'
import {
  createBoundedMediaOutput,
  createMp4PacketMux,
  measureMp4Output,
  reclaimMediaOutputs,
  verifyMp4Audio,
} from '@/shared/lib/media'

type Request = {
  url: string
  fingerprint: string
  ratio: ClipRatio
  mode: 'silent' | 'source' | 'narration' | 'mixed'
  speechUrl: string
  cancelAt?: number
  slowMs?: number
  memory?: boolean
}
declare global {
  interface Window {
    exportFixture: {
      render(request: Request): Promise<unknown>
      storage(): Promise<unknown>
      leaveAbandoned(): Promise<void>
      recover(): Promise<unknown>
    }
  }
}
const namespace = 'postpilot-browser-output-fixture-v1'
window.exportFixture = {
  async render(request) {
    const controller = new AbortController(),
      signal = controller.signal
    const retain = request.mode === 'source' || request.mode === 'mixed'
    const narrated = request.mode === 'narration' || request.mode === 'mixed'
    const plan: ClipEditPlan = {
      nativeComposition: true,
      durationMs: 15000,
      elements: [],
      cuts: Array.from({ length: retain ? 5 : 1 }, (_, i) => ({
        id: `cut-${i}`,
        sourceId: 'source',
        fingerprint: request.fingerprint,
        startMs: retain ? 3000 : 0,
        endMs: retain ? 6000 : 15000,
        transitionMs: 0,
        playbackRatePermille: 1000,
        focal: { x: 0.5, y: 0.5 },
        volumePermille: retain ? 1000 : 0,
        copies: [],
      })),
    }
    let speechBytes: ArrayBuffer | undefined
    if (narrated) {
      speechBytes = await (await fetch(request.speechUrl)).arrayBuffer()
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
    }
    const snapshot = await freezeBrowserComposition({
      ownerId: 'fixture-owner',
      projectId: 'fixture-project',
      projectRevision: 1,
      planRevision: 1,
      plan,
      ratio: request.ratio,
      design: { hideDisclosure: true },
      authoritativeFingerprint: 'a'.repeat(64),
      sources: [
        {
          sourceId: 'source',
          fingerprint: request.fingerprint,
          durationMs: retain ? 7000 : 30000,
          width: retain ? 64 : 320,
          height: retain ? 32 : 180,
          hasAudio: retain,
          allowedRatePermille: [500, 750, 1000, 1250, 1500, 2000],
        },
      ],
    })
    const audio = await renderBrowserAudio(
      plan,
      request.ratio,
      { source: async () => ({ kind: 'url', url: request.url }) },
      signal,
      undefined,
      async () => speechBytes!.slice(0),
    )
    const output = await createBoundedMediaOutput(
      {
        maxBytes: CLIP_BROWSER_RENDER.temporaryOutputBytes,
        pageBytes: CLIP_BROWSER_RENDER.outputPageBytes,
        ...(request.memory ? {} : { temporary: { namespace, identity: crypto.randomUUID() } }),
      },
      signal,
    )
    const mux = await createMp4PacketMux(output, 30, audio, signal)
    let completed = 0
    const render = renderBrowserVideo(
      { snapshot, plan, ratio: request.ratio, assets: [], collectMeasurements: true },
      [],
      async () => request.url,
      signal,
      undefined,
      undefined,
      async (packet, metadata) => {
        if (request.slowMs) await new Promise((resolve) => setTimeout(resolve, request.slowMs))
        await mux.video(packet, metadata)
      },
    )
    const progress = (async () => {
      for await (const frame of render.progress) {
        completed = frame.completedFrames
        if (request.cancelAt && completed >= request.cancelAt) controller.abort()
      }
    })()
    try {
      const video = await render.result
      await progress
      const file = await mux.finalize()
      video.outputMeasurements = await measureMp4Output(file, 30, snapshot.frameCount, signal)
      const decodedAudio = audio
        ? await verifyMp4Audio(
            file,
            {
              sampleFrames: audio.sampleFrames,
              sampleRate: audio.config.sampleRate,
              channels: audio.config.numberOfChannels,
              maxBytes: CLIP_AUDIO_PROCESSING.maxPcmBytes,
              paddingFrames: CLIP_AUDIO_PROCESSING.encoderPaddingFrames,
              maxSampleFrames: CLIP_AUDIO_PROCESSING.maxSampleFrames,
            },
            signal,
          )
        : undefined
      if (audio && decodedAudio) {
        audio.loudnessLUFS = decodedAudio.loudnessLUFS
        audio.truePeakDBTP = decodedAudio.truePeakDBTP
        audio.silent = decodedAudio.silent
      }
      const verdict = browserRenderVerdict(video, audio, request.ratio, plan.durationMs)
      const url = URL.createObjectURL(file)
      const player = document.createElement('video')
      player.muted = true
      player.src = url
      document.body.append(player)
      const wait = (event: string) =>
        new Promise<void>((resolve, reject) => {
          const timer = setTimeout(() => reject(new Error(`VIDEO_${event}_TIMEOUT`)), 10000)
          player.addEventListener(
            event,
            () => {
              clearTimeout(timer)
              resolve()
            },
            { once: true },
          )
          player.addEventListener(
            'error',
            () => {
              clearTimeout(timer)
              reject(new Error('MP4_PLAYBACK_FAILED'))
            },
            { once: true },
          )
        })
      try {
        await wait('loadedmetadata')
        const seeks = []
        for (const time of [0.1, 7.5, 14.9]) {
          const seeked = wait('seeked')
          player.currentTime = time
          await seeked
          seeks.push(player.currentTime)
        }
        await player.play()
        player.pause()
        return {
          mode: request.mode,
          ratio: request.ratio,
          decodedAudio,
          verdict,
          fileBytes: file.size,
          kind: output.kind,
          storedBytes: output.measurements(),
          packetResources: video.packetResources,
          output: video.outputMeasurements,
          frameCount: video.frameCount,
          retainedVideoPackets: video.chunks.length,
          sourceResources: video.sourceResources,
          audioSamples: audio?.sampleFrames,
          primingFrames: audio?.primingFrames,
          speechFingerprint: audio?.speechFingerprint,
          seeks,
          completed,
          qualification: false,
        }
      } finally {
        player.remove()
        URL.revokeObjectURL(url)
      }
    } catch (error) {
      await progress
      return {
        error: error instanceof Error ? error.message : String(error),
        name: error instanceof Error ? error.name : '',
        completed,
        qualification: false,
      }
    } finally {
      render.cancel()
      await mux.cancel()
      await output.dispose()
      if (audio) audio.chunks.length = 0
    }
  },
  async storage() {
    const controller = new AbortController()
    const output = await createBoundedMediaOutput(
      { maxBytes: 16, pageBytes: 4, temporary: { namespace, identity: 'live-owner-a' } },
      controller.signal,
    )
    const writer = output.stream.getWriter()
    await writer.write({ type: 'write', position: 8, data: new Uint8Array([8, 9, 10, 11]) })
    await writer.write({ type: 'write', position: 0, data: new Uint8Array([1, 2, 3, 4, 5]) })
    await writer.write({ type: 'write', position: 3, data: new Uint8Array([13, 14, 15]) })
    await writer.close()
    const file = await output.file()
    await reclaimMediaOutputs(namespace)
    const directory = await (await navigator.storage.getDirectory()).getDirectoryHandle(namespace)
    const liveNames = []
    for await (const [name] of directory.entries()) liveNames.push(name)
    const bytes = [...new Uint8Array(await file.arrayBuffer())]
    await output.dispose()
    const remaining = []
    for await (const [name] of directory.entries()) remaining.push(name)
    const bounded = await createBoundedMediaOutput(
      { maxBytes: 8, pageBytes: 4, temporary: { namespace, identity: 'overflow-owner-b' } },
      new AbortController().signal,
    )
    const pressure = bounded.stream.getWriter()
    let overflow = ''
    try {
      await pressure.write({ type: 'write', position: 8, data: new Uint8Array([1]) })
    } catch (error) {
      overflow = error instanceof Error ? error.message : String(error)
    }
    await bounded.dispose()
    const cancelled = await createBoundedMediaOutput(
      { maxBytes: 8, pageBytes: 4, temporary: { namespace, identity: 'cancelled-owner-b' } },
      controller.signal,
    )
    const cancelWriter = cancelled.stream.getWriter()
    await cancelWriter.write({ type: 'write', position: 0, data: new Uint8Array([1]) })
    controller.abort()
    await cancelled.dispose()
    const afterCancel = []
    for await (const [name] of directory.entries()) afterCancel.push(name)
    return { kind: output.kind, bytes, liveNames, remaining, overflow, afterCancel }
  },
  async leaveAbandoned() {
    const output = await createBoundedMediaOutput(
      { maxBytes: 8, pageBytes: 4, temporary: { namespace, identity: 'abandoned-owner-a' } },
      new AbortController().signal,
    )
    const writer = output.stream.getWriter()
    await writer.write({ type: 'write', position: 0, data: new Uint8Array([42]) })
    await writer.close()
    await output.file()
  },
  async recover() {
    const directory = await (await navigator.storage.getDirectory()).getDirectoryHandle(namespace)
    const before = []
    for await (const [name] of directory.entries()) before.push(name)
    await reclaimMediaOutputs(namespace)
    const after = []
    for await (const [name] of directory.entries()) after.push(name)
    return { before, after }
  },
}
