import { EditorMountedSurface } from './EditorMountedSurface'
import { createRoot, type Root } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { initializeI18n } from '@/app/providers/i18n'
import {
  ClipProjectSchema,
  ClipEditingStateSchema,
  ClipEditPlanSchema,
  ClipPlanService,
  ClipGenerationService,
  ClipRenderService,
  ClipSpeechService,
  SaveClipEditPlanResponseSchema,
  GetClipProjectResponseSchema,
} from '@/shared/api'
import { toClipProject, type ClipProject } from '@/entities/clip-project'
import { clipPlanToProto, type ClipEditPlan } from '@/entities/clip-plan'
import type { useClipCorrection } from '@/features/correct-clip'
import { openOriginalVideo, createAudioRangeReader } from '@/shared/lib'
import { CLIP_VIDEO_DECODING, CLIP_AUDIO_PROCESSING } from '@/entities/clip-preview'
import { localComponentSamples } from '@/entities/clip-preview/model/component-samples'
import { type ClipRatioId } from '@/entities/clip-design'
import '@/app/styles/index.css'

export interface MountedEditorInput {
  url: string
  fingerprint: string
  ratio?: ClipRatioId
  audioMode?: 'silent' | 'source' | 'narration' | 'mixed' | 'missing' | 'stale'
  speech: {
    audioHash: string
    samples: number
    sampleRate: number
    channels: number
    bytes: number
  }
}
let root: Root | undefined,
  project: ClipProject | undefined,
  current: ReturnType<typeof useClipCorrection> | undefined
let stored: ReturnType<typeof create<typeof ClipProjectSchema>>,
  rpcCalls: string[] = [],
  sourceDelay = 0,
  speechDelay = 0,
  missing = false
let originalUrl = '',
  fileUrl: string | undefined,
  sourceAccessCalls = 0
const telemetry = {
  startedNodes: 0,
  stoppedNodes: 0,
  liveNodes: 0,
  contexts: 0,
  nodes: [] as {
    when: number
    offset: number
    duration: number
    sampleRate: number
    samples: number
    playbackRate: number
  }[],
}
const nativeAudioContextConstructor = AudioContext
window.AudioContext = new Proxy(nativeAudioContextConstructor, {
  construct(target, args) {
    telemetry.contexts++
    return Reflect.construct(target, args)
  },
})
const createSource = AudioContext.prototype.createBufferSource
AudioContext.prototype.createBufferSource = function () {
  const node = createSource.call(this),
    start = node.start,
    stop = node.stop
  let live = false
  node.start = function (...args) {
    const value = start.apply(this, args)
    live = true
    telemetry.startedNodes++
    telemetry.liveNodes++
    telemetry.nodes.push({
      when: args[0] ?? 0,
      offset: args[1] ?? 0,
      duration: args[2] ?? node.buffer?.duration ?? 0,
      samples: node.buffer?.length ?? 0,
      sampleRate: node.buffer?.sampleRate ?? 0,
      playbackRate: node.playbackRate.value,
    })
    return value
  }
  const ended = () => {
    if (live) {
      live = false
      telemetry.liveNodes--
      telemetry.stoppedNodes++
    }
  }
  node.stop = function (...args) {
    const value = stop.apply(this, args)
    ended()
    return value
  }
  node.addEventListener('ended', ended)
  return node
}
initializeI18n('en')
const wait = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms))
const resolvePlayback = async () => {
  sourceAccessCalls++
  await wait(sourceDelay)
  if (missing) throw { reason: 'missing' }
  return fileUrl ?? originalUrl
}
export async function mountEditor(input: MountedEditorInput) {
  root?.unmount()
  current = undefined
  project = undefined
  sourceDelay = 0
  speechDelay = 0
  missing = false
  originalUrl = input.url
  sourceAccessCalls = 0
  rpcCalls = []
  fileUrl = undefined
  const signal = new AbortController().signal,
    video = await openOriginalVideo({ kind: 'url', url: input.url }, CLIP_VIDEO_DECODING, signal)
  const metadata = video.metadata
  video.dispose()
  if (metadata.durationFromMetadata === null || metadata.durationFromMetadata < 15)
    throw new Error('Fixture needs15seconds of real original footage')
  const reader = createAudioRangeReader(signal, CLIP_AUDIO_PROCESSING)
  let hasAudio: boolean
  try {
    hasAudio = !!(await reader.decode(
      { kind: 'url', url: input.url },
      { startUs: 0, endUs: 1000000, targetSampleRate: 48000 },
      16 * 1024 * 1024,
    ))
  } finally {
    reader.close()
  }
  const enabled =
    input.audioMode === 'narration' ||
    input.audioMode === 'mixed' ||
    input.audioMode === 'missing' ||
    input.audioMode === 'stale'
  const retain = input.audioMode === 'source' || input.audioMode === 'mixed'
  const speech = {
    ...input.speech,
    assetId: 'synthetic-speech',
    voiceId: 'synthetic-voice',
    bindingDigest: 'binding',
    inputHash: 'script',
    settingsHash: 'settings',
    profileId: 'synthetic-mp3-profile',
    profileRevision: 1,
    timing: [],
  }
  const plan: ClipEditPlan = {
    nativeComposition: true,
    durationMs: 15000,
    sourceAudio: [
      { sourceId: 'source', fingerprint: input.fingerprint, retainOriginalAudio: retain },
    ],
    cuts: [
      {
        id: 'cut',
        sourceId: 'source',
        fingerprint: input.fingerprint,
        startMs: 0,
        endMs: 15000,
        transitionMs: 0,
        volumePermille: 1000,
        playbackRatePermille: 1000,
        copies: [],
      },
    ],
    elements: [
      {
        instanceId: 'caption',
        elementId: 'caption',
        cutId: '',
        kind: 'fixed',
        role: 'caption',
        text: '현재 장면',
        rows: [],
        style: 'bold',
        position: 'auto',
        align: 'center',
        basis: 'output-start',
        startMs: 1000,
        endMs: 8000,
        resolvedStartMs: 1000,
        resolvedEndMs: 8000,
        pace: 'steady',
        accent: '',
        keyword: '',
        groupId: '',
        itemId: '',
      },
    ],
    ...(enabled
      ? {
          narration: {
            enabled: true,
            confirmedVoiceId: 'synthetic-voice',
            bindingDigest: 'binding',
            volumePermille: 750,
            segments: [
              {
                id: 'spoken',
                text: 'Synthetic660Hz MP3 tone',
                textRevision: 1,
                inputHash: input.audioMode === 'stale' ? 'changed-script' : 'script',
                startMs: 1000,
                endMs: 1000 + Math.ceil((speech.samples * 1000) / speech.sampleRate),
                ...(input.audioMode !== 'missing' ? { speech } : {}),
              },
            ],
          },
        }
      : {}),
  }
  stored = create(ClipProjectSchema, {
    id: 'synthetic-editor',
    title: 'Synthetic owned editor',
    ratio: input.ratio ?? 'vertical',
    targetDurationMs: 15000,
    captionPace: 'steady',
    introPreset: 'a',
    outroPreset: 'b',
    allowedCaptionStyles: ['bold'],
    hideDisclosure: true,
    editPlanRevision: 1,
    editing: create(ClipEditingStateSchema, {
      plan: create(ClipEditPlanSchema, clipPlanToProto(plan)),
      sources: [
        {
          id: 'source',
          fingerprint: input.fingerprint,
          filename: 'owned-synthetic.mp4',
          width: metadata.displayWidth,
          height: metadata.displayHeight,
          durationMs: Math.round(metadata.durationFromMetadata * 1000),
          hasAudio,
          originalMeasurementProvenance: 'browser_client',
          allowedRatePermille: [500, 750, 1000, 1250, 1500, 2000],
        },
      ],
      fadeMs: 200,
      maxCuts: 100,
      maxCopyRunes: 500,
      minDurationMs: 15000,
      maxDurationMs: 60000,
    }),
  })
  project = toClipProject(stored)
  const transport = createRouterTransport((router) => {
    router.rpc(ClipPlanService.method.saveClipEditPlan, (request) => {
      rpcCalls.push('SaveClipEditPlan')
      stored = create(ClipProjectSchema, {
        ...stored,
        editPlanRevision: stored.editPlanRevision + 1,
        editing: { ...stored.editing!, plan: request.plan },
      })
      return create(SaveClipEditPlanResponseSchema, { project: stored })
    })
    router.rpc(ClipGenerationService.method.getClipProject, () => {
      rpcCalls.push('GetClipProject')
      return create(GetClipProjectResponseSchema, { project: stored })
    })
    router.rpc(ClipSpeechService.method.getClipSpeechAccess, async () => {
      rpcCalls.push('GetClipSpeechAccess')
      await wait(speechDelay)
      return {
        url: '/clip/speech/' + 'a'.repeat(32),
        audioHash: input.speech.audioHash,
        bytes: BigInt(input.speech.bytes),
        expiresAt: '2040-01-01T00:00:00Z',
      }
    })
    for (const method of [
      ClipPlanService.method.getClipCaptionPreview,
      ClipPlanService.method.getClipCaptionStyleSamples,
      ClipPlanService.method.getClipRegionPresetSamples,
      ClipRenderService.method.prepareClipPreview,
      ClipRenderService.method.prepareClipCaptionFrames,
    ])
      router.rpc(method, () => {
        rpcCalls.push(method.name)
        throw new Error('Forbidden ordinary server raster/frame request:' + method.name)
      })
  })
  const container =
    document.getElementById('editor') ??
    document.body.appendChild(Object.assign(document.createElement('div'), { id: 'editor' }))
  root = createRoot(container)
  root.render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        })
      }
    >
      <TransportProvider transport={transport}>
        <EditorMountedSurface
          initial={project}
          inspect={(value) => {
            current = value
          }}
          resolvePlayback={resolvePlayback}
        />
      </TransportProvider>
    </QueryClientProvider>,
  )
  return { metadata, hasAudio }
}
export function editorState() {
  return {
    timeMs: current?.timeline.timeMs,
    plan: current?.draft,
    dirty: current?.dirty,
    revision: current?.revision,
    sourceAccessCalls,
    rpcCalls: [...rpcCalls],
    telemetry: { ...telemetry, nodes: [...telemetry.nodes] },
    canvas: document.querySelector<HTMLCanvasElement>('[data-clip-local-preview]')?.dataset,
  }
}
export function seekEditor(timeMs: number) {
  current?.dispatch({ type: 'seek', timeMs })
}
export function editEditor(patch: Parameters<NonNullable<typeof current>['change']>[0]) {
  current?.change(patch)
}
export function delays(value: { source?: number; speech?: number; missing?: boolean }) {
  sourceDelay = value.source ?? sourceDelay
  speechDelay = value.speech ?? speechDelay
  missing = value.missing ?? missing
}
export async function componentCatalog(ratio: ClipRatioId) {
  const signal = new AbortController().signal
  const styles = await localComponentSamples.styles(ratio, 'steady', signal),
    rapid = await localComponentSamples.styles(ratio, 'rapid', signal),
    presets = await localComponentSamples.presets(ratio, 'Slot {n}', signal)
  if (
    styles.captions.length !== 16 ||
    rapid.captions.length !== 16 ||
    presets.intro.length !== 8 ||
    presets.outro.length !== 7 ||
    styles.captions.some((caption) => !caption.svg.includes('data:image/png'))
  )
    throw new Error('Incomplete local component catalog')
  return {
    ratio,
    styles: styles.captions.map((caption) => caption.style),
    rapid: rapid.captions.map((caption) => caption.style),
    intro: presets.intro.map((sample) => sample.preset),
    outro: presets.outro.map((sample) => sample.preset),
  }
}
export async function unmountEditor() {
  root?.unmount()
  root = undefined
  current = undefined
  if (fileUrl) URL.revokeObjectURL(fileUrl)
  fileUrl = undefined
  await wait(50)
  return editorState()
}
