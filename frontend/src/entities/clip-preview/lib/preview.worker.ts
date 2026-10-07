import { CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'
import type { ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import { CLIP_LOCAL_PREVIEW } from '../config/local-preview'
import {
  readBrowserCompositionSnapshot,
  evaluateBrowserFlow,
  type BrowserCompositionSnapshot,
  type BrowserEvaluatedFrame,
} from '../model/browser-composition'
import { BrowserLocalComponents } from '../model/local-components'
import { BrowserFootageResources, type BrowserSourceAccess } from '../model/browser-footage'
import {
  measureBrowserBackground,
  type BrowserBackgroundEvidence,
} from '../model/background-sampling'
import { drawMeasuredBrowserComponents } from '../model/background-paint'
import { compositeBrowserFrame } from '../model/composite-video'
import { NativeFadeBlackSurface } from '../model/native-fadeblack'
import type {
  BrowserPreviewWorkerInput,
  BrowserPreviewWorkerOutput,
  BrowserPreviewFrameRequest,
} from '../model/preview-worker-protocol'
import type { BrowserMediaSourceAccess } from '@/shared/lib'

const send = (message: BrowserPreviewWorkerOutput, transfer: Transferable[] = []) =>
  self.postMessage(message, { transfer })
let snapshot: BrowserCompositionSnapshot | undefined,
  components: BrowserLocalComponents | undefined,
  canvas: OffscreenCanvas | undefined,
  context: OffscreenCanvasRenderingContext2D | undefined
let lifetime = new AbortController(),
  frameOwner: AbortController | undefined,
  footage: BrowserFootageResources | undefined,
  background: BrowserBackgroundEvidence | undefined,
  backgroundPending: Promise<BrowserBackgroundEvidence> | undefined,
  fadeBlack: NativeFadeBlackSurface | undefined
let lastFlow = false
let requestId = 0,
  epoch = 0,
  generation = 0,
  lastFrame = -1,
  busy = false
let pending: ({ id: number; epoch: number } & BrowserPreviewFrameRequest) | undefined
const accessWaiters = new Map<
  number,
  { resolve: (value: BrowserMediaSourceAccess) => void; reject: (error: unknown) => void }
>()
const access: BrowserSourceAccess = (sourceId, fingerprint, signal) =>
  new Promise((resolve, reject) => {
    signal.throwIfAborted()
    const id = ++requestId,
      binding = snapshot!.snapshotFingerprint
    const cleanup = () => {
      accessWaiters.delete(id)
      signal.removeEventListener('abort', cancelled)
    }
    const cancelled = () => {
      cleanup()
      reject(signal.reason)
    }
    signal.addEventListener('abort', cancelled, { once: true })
    accessWaiters.set(id, {
      resolve: (value) => {
        cleanup()
        resolve(value)
      },
      reject: (error) => {
        cleanup()
        reject(error)
      },
    })
    send({ type: 'source', id, sourceId, fingerprint, snapshotFingerprint: binding })
  })
async function initialize(id: number, value: unknown) {
  const own = ++generation
  lifetime.abort()
  lifetime = new AbortController()
  frameOwner?.abort()
  await footage?.dispose()
  footage = undefined
  fadeBlack?.close()
  fadeBlack = undefined
  background = undefined
  backgroundPending = undefined
  lastFrame = -1
  const next = await readBrowserCompositionSnapshot(value)
  if (own !== generation) return
  if (
    components &&
    snapshot?.ownerId === next.ownerId &&
    snapshot.projectId === next.projectId &&
    JSON.stringify(snapshot.versions) === JSON.stringify(next.versions)
  )
    components.updateSnapshot(next)
  else {
    components?.destroy()
    components = new BrowserLocalComponents(next)
  }
  snapshot = next
  const size = CLIP_DESIGN.ratios[next.ratio].canvas
  if (canvas) {
    canvas.width = 0
    canvas.height = 0
  }
  canvas = new OffscreenCanvas(
    Math.round(size.width * CLIP_LOCAL_PREVIEW.scale),
    Math.round(size.height * CLIP_LOCAL_PREVIEW.scale),
  )
  context = canvas.getContext('2d', { alpha: false }) ?? undefined
  if (!context) throw new Error('CLIP_CANVAS_UNAVAILABLE')
  context.scale(CLIP_LOCAL_PREVIEW.scale, CLIP_LOCAL_PREVIEW.scale)
  await components.resolveLayout(lifetime.signal)
  if (own !== generation) return
  send({ type: 'initialized', id })
}
async function render(request: { id: number; epoch: number } & BrowserPreviewFrameRequest) {
  if (!snapshot || !components || !context || !canvas) throw new Error('CLIP_PREVIEW_NOT_READY')
  const current = snapshot,
    local = components,
    target = canvas,
    ctx = context,
    ownGeneration = generation
  const check = () => {
    if (request.epoch !== epoch || current !== snapshot || ownGeneration !== generation)
      throw new Error('CLIP_SNAPSHOT_SUPERSEDED')
    frameOwner!.signal.throwIfAborted()
  }
  if (request.flow !== lastFlow || (!request.flow && request.frame < lastFrame) || !footage) {
    await footage?.dispose()
    footage = undefined
    frameOwner?.abort()
    frameOwner = new AbortController()
    footage = new BrowserFootageResources(current, access, frameOwner.signal)
  }
  check()
  let measured = background
  if (!request.flow) {
    backgroundPending ??= measureBrowserBackground(
      current,
      local,
      access,
      frameOwner!.signal,
    ).catch((error) => {
      if (current === snapshot) backgroundPending = undefined
      throw error
    })
    measured = await backgroundPending
    check()
    background = measured
  }
  let displayed: { cutId: string; sourceMs: number; outputMs: number }[] = []
  const frameFootage = footage
  await compositeBrowserFrame(
    { snapshot: current, plan: current.plan as ClipEditPlan, ratio: current.ratio, assets: [] },
    {
      context: ctx,
      assertCurrent: check,
      displayed: (value) => {
        displayed = [...value]
      },
      footage: {
        prepare: async (evaluated: BrowserEvaluatedFrame) => {
          let state = evaluated
          if (request.flow) state = evaluateBrowserFlow(current, request.timeMs ?? evaluated.timeMs)
          try {
            return await frameFootage.prepare(state)
          } catch (error) {
            if (!request.flow) throw error
            check()
            return []
          }
        },
      },
      nativeFadeBlack: (rest) => {
        if (request.flow) return
        fadeBlack ??= new NativeFadeBlackSurface(target.width, target.height)
        ctx.save()
        ctx.resetTransform()
        fadeBlack.apply(target, ctx, rest)
        ctx.restore()
      },
      local: async (evaluated) => {
        const state = request.flow
          ? evaluateBrowserFlow(current, request.timeMs ?? evaluated.timeMs)
          : evaluated
        const resources = await local.prepare(
          state,
          frameOwner!.signal,
          (instanceId) => ({
            accent:
              CLIP_DESIGN.accent[
                (state.components.find((component) => component.component.instanceId === instanceId)
                  ?.component.element.accent ?? '') as keyof typeof CLIP_DESIGN.accent
              ],
            accentWhite: measured?.measurements.some(
              (m) =>
                m.instanceId === instanceId &&
                m.phraseIndex ===
                  (state.components.find(
                    (component) => component.component.instanceId === instanceId,
                  )?.phraseIndex ?? 0) &&
                m.accentWhite,
            ),
          }),
          { representative: request.flow, position: request.captionPosition },
        )
        try {
          check()
          if (measured && !request.flow)
            drawMeasuredBrowserComponents(ctx, current, state, resources, measured)
          else resources.forEach((resource) => resource.draw(ctx))
        } finally {
          resources.forEach((resource) => resource.close())
        }
      },
      source: async () => {
        throw new Error('CLIP_SOURCE_UNAVAILABLE')
      },
      asset: async () => {
        throw new Error('CLIP_BACKGROUND_FOREIGN_ASSETS')
      },
      captionFrame: async () => {
        throw new Error('CLIP_BACKGROUND_FOREIGN_ASSETS')
      },
      releaseAssets: () => {},
    },
    request.frame,
  )
  check()
  lastFrame = request.frame
  lastFlow = request.flow
  const stats = footage.measurements(),
    bitmap = target.transferToImageBitmap()
  send(
    {
      type: 'rendered',
      id: request.id,
      epoch: request.epoch,
      result: {
        bitmap,
        captionPosition: request.captionPosition,
        frame: request.frame,
        flow: request.flow,
        fingerprint: current.snapshotFingerprint,
        displayed,
        resources: {
          activeCuts: stats.activeCuts,
          liveFrames: stats.presentation.liveFrames,
          decoderReservedBytes: stats.decoderReservedBytes,
        },
        backgroundMeasured: !!measured && !request.flow,
      },
    },
    [bitmap],
  )
}
async function drain() {
  if (busy) return
  busy = true
  try {
    while (pending) {
      const request = pending
      pending = undefined
      try {
        await render(request)
      } catch (error) {
        send({
          type: 'failed',
          id: request.id,
          error: error instanceof Error ? error.message : 'CLIP_PREVIEW_FAILED',
        })
      }
    }
  } finally {
    busy = false
  }
}
self.onmessage = (event: MessageEvent<BrowserPreviewWorkerInput>) => {
  const message = event.data
  if (message.type === 'source') {
    const waiter = accessWaiters.get(message.id)
    if (message.access) waiter?.resolve(message.access)
    else waiter?.reject(new Error(message.error ?? 'CLIP_SOURCE_UNAVAILABLE'))
    return
  }
  if (message.type === 'initialize') {
    void initialize(message.id, message.snapshot).catch((error) =>
      send({
        type: 'failed',
        id: message.id,
        error: error instanceof Error ? error.message : 'CLIP_PREVIEW_FAILED',
      }),
    )
    return
  }
  if (message.type === 'cancel') {
    epoch = message.epoch
    frameOwner?.abort()
    void footage?.dispose()
    footage = undefined
    pending = undefined
    return
  }
  if (message.type === 'frame') {
    epoch = message.epoch
    if (pending) send({ type: 'failed', id: pending.id, error: 'CLIP_SNAPSHOT_SUPERSEDED' })
    pending = message
    void drain()
    return
  }
  generation++
  lifetime.abort()
  frameOwner?.abort()
  void footage?.dispose()
  components?.destroy()
  fadeBlack?.close()
  if (canvas) {
    canvas.width = 0
    canvas.height = 0
  }
  self.close()
}
