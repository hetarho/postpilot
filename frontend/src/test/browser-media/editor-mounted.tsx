import { ThemeProvider, bootstrapTheme } from '@/app/providers/theme'
import { createRouter, createMemoryHistory, RouterProvider } from '@tanstack/react-router'
import { routeTree } from '@/app/routes/router'
import { createFakeAuthTransport } from '@/test/session'
import { drawMeasuredBrowserComponents } from '@/entities/clip-preview/model/background-paint'
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
import {
  projectBrowserComposition,
  freezeBrowserComposition,
  freezeBrowserPreviewComposition,
  evaluateBrowserFrame,
  BrowserLocalComponents,
  BrowserFootageResources,
  BrowserPreviewWorker,
  measureBrowserBackground,
  compositeBrowserFrame,
  BrowserCompositionPlayback,
  CLIP_VIDEO_DECODING,
  CLIP_AUDIO_PROCESSING,
} from '@/entities/clip-preview'
import { localComponentSamples } from '@/entities/clip-preview/model/component-samples'
import { type ClipRatioId } from '@/entities/clip-design'
import '@/app/styles/index.css'

export interface MountedEditorInput {
  url: string
  fingerprint: string
  ratio?: ClipRatioId
  rapid?: boolean
  sourceDelay?: number
  speechDelay?: number
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
  clockStarts: 0,
  pendingPlays: 0,
  nodes: [] as {
    when: number
    offset: number
    duration: number
    sampleRate: number
    samples: number
    playbackRate: number
  }[],
}
const nativePlay = BrowserCompositionPlayback.prototype.play
BrowserCompositionPlayback.prototype.play = async function (...args) {
  telemetry.pendingPlays++
  try {
    const started = await nativePlay.apply(this, args)
    if (started) telemetry.clockStarts++
    return started
  } finally {
    telemetry.pendingPlays--
  }
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
  sourceDelay = input.sourceDelay ?? 0
  speechDelay = input.speechDelay ?? 0
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
        text: input.rapid ? '첫 문구 다음 문구' : '현재 장면',
        ...(input.rapid
          ? {
              phrases: [
                { text: '첫 문구', startMs: 1000, endMs: 3000 },
                { text: '다음 문구', startMs: 3000, endMs: 8000 },
              ],
            }
          : {}),
        rows: [],
        style: 'bold',
        position: 'auto',
        align: 'center',
        basis: 'output-start',
        startMs: 1000,
        endMs: 8000,
        resolvedStartMs: 1000,
        resolvedEndMs: 8000,
        pace: input.rapid ? 'rapid' : 'steady',
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

/** Full export-purpose one-frame draw versus the actual reduced preview Worker at explicit frames. */
export async function frameParity(frame: number) {
  if (!current || !project) throw new Error('No mounted editor')
  const input = projectBrowserComposition({
    ownerId: 'synthetic-editor-owner',
    projectId: project.id,
    projectRevision: project.editPlanRevision,
    planRevision: current.revision,
    ratio: project.ratio,
    plan: current.previewPlan,
    sources: project.editing!.sources,
    design: {
      captionStyles: project.allowedCaptionStyles,
      captionPace: project.captionPace,
      accent: project.accent,
      introPreset: project.introPreset,
      outroPreset: project.outroPreset,
      hideDisclosure: true,
    },
  })
  const full = await freezeBrowserComposition(input),
    preview = await freezeBrowserPreviewComposition(input),
    signal = new AbortController().signal
  if (
    JSON.stringify(evaluateBrowserFrame(full, frame)) !==
    JSON.stringify(evaluateBrowserFrame(preview, frame))
  )
    throw new Error('Preview/export frame evaluator mismatch')
  const access = async () => ({ kind: 'url' as const, url: originalUrl }),
    components = new BrowserLocalComponents(full),
    footage = new BrowserFootageResources(full, access, signal),
    worker = new BrowserPreviewWorker(access)
  const size = {
      width: full.ratio === 'horizontal' ? 1920 : 1080,
      height: full.ratio === 'vertical' ? 1920 : 1080,
    },
    canvas = new OffscreenCanvas(size.width, size.height),
    context = canvas.getContext('2d')!
  try {
    await components.resolveLayout(signal)
    const evidence = await measureBrowserBackground(full, components, access, signal)
    await compositeBrowserFrame(
      { snapshot: full, plan: full.plan as ClipEditPlan, ratio: full.ratio, assets: [] },
      {
        context,
        footage,
        local: async (state) => {
          const resources = await components.prepare(state, signal, (id) => ({
            accentWhite: evidence.measurements.some(
              (m) =>
                m.instanceId === id &&
                m.phraseIndex ===
                  (state.components.find((value) => value.component.instanceId === id)
                    ?.phraseIndex ?? 0) &&
                m.accentWhite,
            ),
          }))
          try {
            drawMeasuredBrowserComponents(context, full, state, resources, evidence)
          } finally {
            resources.forEach((resource) => resource.close())
          }
        },
        source: async () => {
          throw new Error('Unexpected legacy source')
        },
        asset: async () => {
          throw new Error('Unexpected server raster')
        },
        captionFrame: async () => {
          throw new Error('Unexpected server frame')
        },
        releaseAssets: () => {},
      },
      frame,
    )
    await worker.initialize(preview, signal)
    const result = await worker.render({ frame, flow: false }, signal),
      small = new OffscreenCanvas(result.bitmap.width, result.bitmap.height),
      reference = new OffscreenCanvas(result.bitmap.width, result.bitmap.height),
      ctx = small.getContext('2d')!,
      ref = reference.getContext('2d')!
    try {
      ctx.drawImage(result.bitmap, 0, 0)
      ref.drawImage(canvas, 0, 0, reference.width, reference.height)
      const actual = ctx.getImageData(0, 0, small.width, small.height).data,
        expected = ref.getImageData(0, 0, reference.width, reference.height).data
      let difference = 0,
        changed = 0
      for (let index = 0; index < actual.length; index++) {
        const delta = Math.abs(actual[index]! - expected[index]!)
        difference += delta
        if (delta > 16) changed++
      }
      const meanAbsoluteDifference = difference / actual.length,
        materiallyDifferent = changed / actual.length
      if (
        meanAbsoluteDifference > 2 ||
        materiallyDifferent > 0.02 ||
        result.resources.liveFrames !== 0
      )
        throw new Error(
          'Reduced preview/export drawing differs:' +
            JSON.stringify({
              meanAbsoluteDifference,
              materiallyDifferent,
              resources: result.resources,
            }),
        )
      return {
        frame,
        evaluatorEqual: true,
        fullPurpose: full.purpose,
        previewPurpose: preview.purpose,
        meanAbsoluteDifference,
        materiallyDifferent,
        resources: result.resources,
        fullSnapshotFingerprint: full.snapshotFingerprint,
        previewSnapshotFingerprint: preview.snapshotFingerprint,
      }
    } finally {
      result.bitmap.close()
      small.width = 0
      small.height = 0
      reference.width = 0
      reference.height = 0
    }
  } finally {
    worker.dispose()
    await footage.dispose()
    components.destroy()
    canvas.width = 0
    canvas.height = 0
  }
}

/** Actual production finalized workspace, with a saved-file fixture and no draft source resolver. */
export async function mountFinalizedEditor() {
  if (!project) throw new Error('No fixture project')
  root?.unmount()
  current = undefined
  rpcCalls = []
  sourceAccessCalls = 0
  const finalized = {
    ...project,
    finalized: {
      at: '2026-10-07T00:00:00Z',
      planRevision: project.editPlanRevision,
      resultId: 'saved-fixture',
    },
    renderedPlanRevision: project.editPlanRevision,
    result: {
      id: 'saved-fixture',
      contentType: 'video/mp4',
      bytes: 5,
      durationMs: 15000,
      createdAt: '2026-10-07T00:00:00Z',
      viewUrl: originalUrl.replace('/source', '/saved-result'),
      downloadUrl: originalUrl.replace('/source', '/saved-result'),
    },
  }
  const transport = createFakeAuthTransport({
      user: { id: 'synthetic-editor-owner' },
      clips: { projects: [finalized], calls: rpcCalls },
    }),
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    }),
    router = createRouter({
      routeTree,
      context: { queryClient, transport },
      history: createMemoryHistory({ initialEntries: ['/clips/' + finalized.id] }),
    })
  root = createRoot(document.getElementById('editor')!)
  root.render(
    <ThemeProvider initialSnapshot={bootstrapTheme()}>
      <TransportProvider transport={transport}>
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </TransportProvider>
    </ThemeProvider>,
  )
  return { resultUrl: finalized.result.viewUrl }
}
export async function blankPresetSlots() {
  if (!current || !project) throw new Error('No mounted editor')
  const plan = structuredClone(current.previewPlan)
  const base = plan.elements![0]!
  plan.elements = [
    {
      ...base,
      instanceId: 'blank-hook',
      elementId: 'blank-hook',
      role: 'hook',
      text: '',
      rows: [],
      phrases: [],
      pace: 'steady',
      ownerStyle: undefined,
    },
    {
      ...base,
      instanceId: 'blank-ending',
      elementId: 'blank-ending',
      role: 'ending',
      text: '',
      rows: [],
      phrases: [],
      pace: 'steady',
      ownerStyle: undefined,
    },
  ]
  const snapshot = await freezeBrowserPreviewComposition(
      projectBrowserComposition({
        ownerId: 'synthetic-editor-owner',
        projectId: project.id,
        projectRevision: 1,
        planRevision: 1,
        ratio: project.ratio,
        plan,
        sources: project.editing!.sources,
        design: {
          captionStyles: ['bold'],
          captionPace: 'steady',
          introPreset: 'cover',
          outroPreset: 'stamp',
          hideDisclosure: true,
        },
      }),
    ),
    local = new BrowserLocalComponents(snapshot)
  try {
    await local.resolveLayout()
    const state = evaluateBrowserFrame(snapshot, 45),
      resources = await local.prepare(state, new AbortController().signal)
    try {
      if (state.components.length || resources.length)
        throw new Error('Blank preset slots invented visible text')
      return {
        components: state.components.length,
        drawnResources: resources.length,
        purpose: snapshot.purpose,
      }
    } finally {
      resources.forEach((value) => value.close())
    }
  } finally {
    local.destroy()
  }
}
