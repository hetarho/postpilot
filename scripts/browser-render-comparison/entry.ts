import { freezeBrowserComposition } from '@/entities/clip-preview/model/browser-composition'
import { renderBrowserAudio } from '@/features/render-clip-browser/api/render-audio'
import { browserAudioPreflight } from '@/features/render-clip-browser/model/audio-preflight'
import { browserRenderVerdict } from '@/features/render-clip-browser/model/verdict'
import type { ClipEditPlan } from '@/entities/clip-plan'
import type { BrowserAudioTrack } from '@/features/render-clip-browser/api/render-audio'
import { type Manifest, type Fixture, type Arm, Phases, flags, sha256, shaJson } from './contract'
import { observeProductionAudio } from './audio-observer'

declare global {
  interface Window {
    candidateArtifact(name: string, ordinal: number, base64: string, final: boolean): Promise<{ bytes: number; sha256: string } | null>
    benchmarkCandidate: {
      environment(): unknown
      run(fixture: Fixture, arm: Arm, temperature: 'cold' | 'warm'): Promise<unknown>
      cleanup(): Promise<unknown>
      compare(left: string, right: string): Promise<unknown>
    }
  }
}
const manifest: Manifest = await (await fetch('/candidate-manifest.json')).json()
const worker = new Worker(new URL('./worker.ts', import.meta.url), { type: 'module' })
let nextId = 0
const pending = new Map<number, { resolve: (value: any) => void; reject: (reason: unknown) => void }>()
worker.onmessage = ({ data }) => {
  const waiter = pending.get(data.id)
  if (!waiter) return
  pending.delete(data.id)
  if (data.error) waiter.reject(Error(data.error))
  else waiter.resolve(data.value)
}
worker.onerror = event => {
  for (const waiter of pending.values()) waiter.reject(Error(event.message || 'BENCHMARK_WORKER_ERROR'))
  pending.clear()
}
const call = (kind: string, value?: unknown, transfer: Transferable[] = []) =>
  new Promise<any>((resolve, reject) => {
    const id = ++nextId
    pending.set(id, { resolve, reject })
    worker.postMessage({ id, kind, value }, transfer)
  })

function planFor(fixture: Fixture): ClipEditPlan {
  const s = manifest.source
  const ranges = [
    { startMs: 0, endMs: 6000, playbackRatePermille: 1000, transitionMs: 0 },
    { startMs: 7000, endMs: 16000, playbackRatePermille: 2000, transitionMs: 200 },
    { startMs: 16000, endMs: 18500, playbackRatePermille: 500, transitionMs: 300 },
  ]
  const intervals = [[1000, 3000], [6000, 8500], [11500, 14500]]
  const plan: ClipEditPlan = {
    nativeComposition: true, durationMs: 15000,
    cuts: ranges.map((range, i) => ({
      ...range, id: `cut-${i}`, sourceId: 'source', fingerprint: s.sha256,
      focal: { x: 0.5, y: 0.5 }, volumePermille: 1000, copies: [],
    })),
    elements: intervals.map(([startMs, endMs], i) => ({
      instanceId: `caption-${i}`, elementId: `caption-${i}`, cutId: '', groupId: '', itemId: '',
      kind: 'fixed', role: 'caption', text: '오늘의 장면 기록\nAV 12,500 원', rows: [],
      style: fixture.style, keyword: '장면', position: 'lower_mid', align: 'center',
      basis: 'output-start', startMs, endMs, resolvedStartMs: startMs, resolvedEndMs: endMs,
      pace: 'steady', accent: 'cyan',
    })),
  }
  if (fixture.narrated) {
    const speech = manifest.speech
    plan.narration = {
      enabled: true, confirmedVoiceId: 'synthetic-tone', bindingDigest: 'diagnostic-only', volumePermille: 700,
      segments: [{
        id: 'synthetic', text: 'synthetic tone', textRevision: 1, inputHash: 'diagnostic-only', startMs: 500, endMs: 1500,
        speech: {
          assetId: 'synthetic', voiceId: 'synthetic-tone', bindingDigest: 'diagnostic-only', inputHash: 'diagnostic-only',
          settingsHash: 'diagnostic-only', audioHash: speech.sha256, profileId: 'diagnostic-only', profileRevision: 1,
          samples: speech.samples, sampleRate: speech.sampleRate, channels: speech.channels, timing: [],
        },
      }],
    }
  }
  return plan
}
async function artifact(name: string, blob: Blob) {
  let result: { bytes: number; sha256: string } | null = null
  let ordinal = 0
  for (let start = 0; start < blob.size; start += manifest.limits.artifactChunkBytes) {
    const bytes = new Uint8Array(await blob.slice(start, start + manifest.limits.artifactChunkBytes).arrayBuffer())
    let binary = ''
    for (let i = 0; i < bytes.length; i += 8192) binary += String.fromCharCode(...bytes.subarray(i, i + 8192))
    await window.candidateArtifact(name, ordinal++, btoa(binary), false)
  }
  result = await window.candidateArtifact(name, ordinal, '', true)
  if (!result || result.bytes !== blob.size) throw Error('BENCHMARK_ARTIFACT_SIZE_MISMATCH')
  return result
}
window.benchmarkCandidate = {
  environment() {
    const canvas = document.createElement('canvas'), gl = canvas.getContext('webgl2')
    const ext = gl?.getExtension('WEBGL_debug_renderer_info')
    const renderer = ext ? gl!.getParameter(ext.UNMASKED_RENDERER_WEBGL) : null
    gl?.getExtension('WEBGL_lose_context')?.loseContext()
    return {
      userAgent: navigator.userAgent, hardwareConcurrency: navigator.hardwareConcurrency,
      reportedDeviceMemoryGB: (navigator as Navigator & { deviceMemory?: number }).deviceMemory ?? null,
      renderer, crossOriginIsolated, actualCodecHardwareUse: 'unverified', ...flags,
    }
  },
  async run(fixture, arm, temperature) {
    const phases = new Phases(), started = performance.now(), plan = planFor(fixture)
    const snapshot = await phases.measure('freezeSnapshot', () => freezeBrowserComposition({
      ownerId: 'external-diagnostic', projectId: fixture.id, projectRevision: 1, planRevision: 1,
      plan, ratio: fixture.ratio, design: { hideDisclosure: true, captionPace: 'steady', captionStyles: [fixture.style] },
      authoritativeFingerprint: manifest.manifestSha256,
      sources: [{
        sourceId: 'source', fingerprint: manifest.source.sha256, durationMs: manifest.source.durationMs,
        width: manifest.source.width, height: manifest.source.height, hasAudio: manifest.source.hasAudio,
        originalMeasurementProvenance: 'browser_client', allowedRatePermille: [500, 1000, 2000],
      }],
    }))
    const controller = new AbortController()
    const deadline = setTimeout(() => { controller.abort(Error('BENCHMARK_RUN_TIMEOUT')); void call('cancel').catch(() => undefined) }, manifest.limits.runTimeoutMs)
    let audio: BrowserAudioTrack | undefined
    let peakMainThreadJSHeapBytes: number | null = null
    const sampler = setInterval(() => {
      const value = (performance as Performance & { memory?: { usedJSHeapSize: number } }).memory?.usedJSHeapSize
      if (value !== undefined) peakMainThreadJSHeapBytes = Math.max(peakMainThreadJSHeapBytes ?? 0, value)
    }, 25)
    try {
      const reservation = browserAudioPreflight(plan)
      const observer = observeProductionAudio()
      let audioObservation
      try {
        audio = await phases.measure('audioPrepareNormalizeEncode', () => renderBrowserAudio(
        plan, fixture.ratio,
        { source: async fingerprint => {
          if (fingerprint !== manifest.source.sha256) throw Error('BENCHMARK_SOURCE_IDENTITY')
          return { kind: 'url' as const, url: location.origin + '/candidate-source' }
        } },
        controller.signal, undefined,
        async (speech, signal) => {
          if (speech.audioHash !== manifest.speech.sha256) throw Error('BENCHMARK_SPEECH_IDENTITY')
          const response = await fetch('/candidate-speech', { signal })
          if (!response.ok) throw Error('BENCHMARK_SPEECH_UNAVAILABLE')
          const bytes = await response.arrayBuffer()
          if (bytes.byteLength !== manifest.speech.bytes || await sha256(bytes) !== manifest.speech.sha256)
            throw Error('BENCHMARK_SPEECH_CHANGED')
          return bytes
        },
        ))
        audioObservation = await phases.measure('audioObserverFinalizeDiagnostic', () => observer.finish())
      } finally { observer.restore() }
      if (audioObservation?.unresolvedRequests || audioObservation?.liveWorkersAfterProductionClose || audioObservation?.normalizedPcm.length !== 1)
        throw Error('BENCHMARK_AUDIO_OBSERVATION_INCOMPLETE')
      const audioEncodedIdentity = audio ? await phases.measure('audioIdentityDiagnostic', async () => ({
        sampleFrames: audio!.sampleFrames, primingFrames: audio!.primingFrames, speechFingerprint: audio!.speechFingerprint,
        packetCount: audio!.chunks.length, encodedBytes: audio!.chunks.reduce((n, c) => n + c.data.byteLength, 0),
        packetDigest: await shaJson(await Promise.all(audio!.chunks.map(async c => ({
          type: c.type, timestamp: c.timestamp, duration: c.duration, sha256: await sha256(c.data),
        })))),
      })) : undefined
      const observedMainPreparationMs = performance.now() - started
      const run = await call('run', {
        arm, temperature, fixture, manifest, snapshot, audio, audioEncodedIdentity,
        sourceUrl: location.origin + '/candidate-source',
      }, audio?.chunks.map(c => c.data.buffer) ?? [])
      const elapsedThroughWorkerVerificationMs = performance.now() - started
      const report = run.report
      report.mainThreadPhases = phases.snapshot()
      report.mainThreadPeakJSHeapBytes = peakMainThreadJSHeapBytes
      report.audioPcm = {
        reservationBytes: reservation.reservedBytes,
        rangeAdmissionBytes: reservation.rangePcmBytes,
        observedPeakSelectedRangeBytes: audio?.sourceResources?.peakRangePcmBytes ?? null,
        sampleFrames: audio?.sampleFrames ?? 0, channels: audio?.config.numberOfChannels ?? 0,
        fullMixDspPrivatePeakBytes: null,
      }
      report.audioSourceResources = audio?.sourceResources
      report.audioObservation = audioObservation
      if (audio && report.decodedAudio) Object.assign(audio, report.decodedAudio)
      report.verdict = await phases.measure('producingVerdict', () => browserRenderVerdict(report.video, audio, fixture.ratio, plan.durationMs))
      if (!report.verdict.passed) throw Error('BENCHMARK_PRODUCING_VERDICT_FAILED')
      const prefix = `${fixture.id}--${arm}--${temperature}`
      report.artifacts = await phases.measure('artifactHashTransferDiagnostic', async () => {
        const mp4 = await artifact(prefix + '.mp4', run.file), samples = []
        for (const sample of run.samples as { frame: number; kind: string; blob: Blob; rawSha256: string }[]) {
          const name = prefix + `--${sample.kind}-${sample.frame}.png`
          samples.push({ frame: sample.frame, kind: sample.kind, rawSha256: sample.rawSha256,
            name, ...await artifact(name, sample.blob) })
        }
        return { mp4, samples }
      })
      report.cleanup = await call('release')
      report.elapsedThroughWorkerVerificationMs = elapsedThroughWorkerVerificationMs
      report.sequentialPreparationPlusWorkerAssemblyMs = observedMainPreparationMs + report.assembledAtMs
      report.elapsedIncludingDiagnosticsMs = performance.now() - started
      report.mainThreadPhases = phases.snapshot()
      report.mainThreadPeakJSHeapBytes = peakMainThreadJSHeapBytes
      return { status: 'done', ...report, ...flags }
    } finally {
      clearTimeout(deadline); clearInterval(sampler)
      controller.abort()
      audio?.chunks.splice(0)
      await call('release').catch(() => undefined)
    }
  },
  async cleanup() {
    try { return await call('cleanup') } finally { worker.terminate() }
  },
  async compare(left, right) {
    const canvas = new OffscreenCanvas(1, 1), ctx = canvas.getContext('2d', { willReadFrequently: true })!
    const bitmaps = await Promise.all([left, right].map(async name => {
      const response = await fetch('/candidate-artifact/' + encodeURIComponent(name))
      if (!response.ok) throw Error('BENCHMARK_COMPARISON_SAMPLE_MISSING')
      return createImageBitmap(await response.blob())
    }))
    try {
      if (bitmaps[0].width !== bitmaps[1].width || bitmaps[0].height !== bitmaps[1].height)
        throw Error('BENCHMARK_COMPARISON_SIZE')
      canvas.width = bitmaps[0].width; canvas.height = bitmaps[0].height
      ctx.drawImage(bitmaps[0], 0, 0); const a = ctx.getImageData(0, 0, canvas.width, canvas.height).data
      ctx.drawImage(bitmaps[1], 0, 0); const b = ctx.getImageData(0, 0, canvas.width, canvas.height).data
      let changedChannels = 0, sum = 0, maxChannelError = 0
      for (let i = 0; i < a.length; i++) {
        const difference = Math.abs(a[i] - b[i])
        changedChannels += Number(difference !== 0); sum += difference; maxChannelError = Math.max(maxChannelError, difference)
      }
      return { changedChannels, meanChannelError: sum / a.length, maxChannelError, channels: a.length, qualification: false }
    } finally { bitmaps.forEach(b => b.close()); canvas.width = canvas.height = 0 }
  },
}
