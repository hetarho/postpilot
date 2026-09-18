import { createClient, type Transport } from '@connectrpc/connect'
import {
  clipRenderNeedsAudio,
  toClipProject,
  type ClipProject,
  type ClipEditPlan,
  type ClipRatio,
  type BrowserVideoTrack,
} from '@/entities/clip-project'
import { ClipService, ClipRenderKind } from '@/shared/api'
import { CLIP_BROWSER_RENDER } from '@/shared/config'
import { BrowserOriginals } from '../lib/originals'
import { prepareBrowserRenderAssets } from './prepare-assets'
import { renderBrowserVideo } from './render-video'
import { renderBrowserAudio, type BrowserAudioTrack } from './render-audio'
import { createBrowserResultStore, storeBrowserResult } from './store-result'

export interface BrowserRenderInput {
  projectId: string
  revision: number
  batchId: string
  plan: ClipEditPlan
  ratio: ClipRatio
  localSources: readonly { fingerprint: string; url: string }[]
  resolvePlayback: (fingerprint: string) => Promise<string>
}
export interface BrowserRenderProgress {
  stage: 'encoding' | 'storing'
  percent: number
}
export interface BrowserRenderOperations {
  admit(input: BrowserRenderInput): Promise<string>
  cancel(id: string): Promise<boolean>
  refresh(projectId: string): Promise<ClipProject>
  prepare: (
    input: BrowserRenderInput,
    signal: AbortSignal,
  ) => ReturnType<typeof prepareBrowserRenderAssets>
  video: typeof renderBrowserVideo
  audio: typeof renderBrowserAudio
  store: (
    id: string,
    video: BrowserVideoTrack,
    audio: BrowserAudioTrack | undefined,
    input: BrowserRenderInput,
    signal: AbortSignal,
    progress: (percent: number) => void,
  ) => Promise<ClipProject>
}
export function browserRenderOperations(transport: Transport): BrowserRenderOperations {
  const client = createClient(ClipService, transport)
  return {
    async admit(input) {
      // Settle admission even if the page leaves, so a late identity can be cancelled.
      const response = await client.startClipRender({
        projectId: input.projectId,
        expectedRevision: input.revision,
        batchId: input.batchId,
        renderKind: ClipRenderKind.BROWSER,
      })
      if (!response.renderId) throw new Error('Missing browser render identity')
      return response.renderId
    },
    cancel: async (renderId) => (await client.cancelClipBrowserRender({ renderId })).cancelled,
    async refresh(id) {
      const response = await client.getClipProject({ id })
      if (!response.project) throw new Error('Missing clip project')
      return toClipProject(response.project)
    },
    prepare: (input, signal) =>
      prepareBrowserRenderAssets(transport, input.projectId, input.revision, input.plan, signal),
    video: renderBrowserVideo,
    audio: renderBrowserAudio,
    store: (id, video, audio, input, signal, progress) =>
      storeBrowserResult(
        id,
        video,
        audio,
        input.ratio,
        input.plan.durationMs,
        createBrowserResultStore(transport),
        signal,
        progress,
      ),
  }
}

/** One page-owned run. Any failure stops its sibling worker before releasing
 * originals; cancellation also fences promotion on the server. */
export async function runBrowserRender(
  input: BrowserRenderInput,
  operations: BrowserRenderOperations,
  signal: AbortSignal,
  progress: (value: BrowserRenderProgress) => void,
): Promise<ClipProject> {
  const controller = new AbortController()
  let id: string | undefined
  let cancellation: Promise<boolean> | undefined
  const cancelAttempt = () => {
    if (id) {
      cancellation ??= operations.cancel(id)
      void cancellation.catch(() => undefined)
    }
    return cancellation
  }
  const abort = () => {
    controller.abort()
    cancelAttempt()
  }
  signal.addEventListener('abort', abort, { once: true })
  const originals = new BrowserOriginals(
    input.localSources,
    input.resolvePlayback,
    controller.signal,
  )
  let assets: Awaited<ReturnType<BrowserRenderOperations['prepare']>> | undefined
  let handle: ReturnType<typeof renderBrowserVideo> | undefined
  let video: BrowserVideoTrack | undefined, audio: BrowserAudioTrack | undefined
  const jobs: Promise<unknown>[] = []
  let picture = 0,
    sound = 0,
    last = 0
  const needsAudio = clipRenderNeedsAudio(input.plan)
  const encoded = () => {
    if (controller.signal.aborted) return
    last = Math.max(
      last,
      (CLIP_BROWSER_RENDER.encodeProgressPercent * (picture + (needsAudio ? sound : 0))) /
        (needsAudio ? 2 : 1),
    )
    progress({ stage: 'encoding', percent: last })
  }
  const stopSibling = (error: unknown): never => {
    controller.abort()
    throw error
  }
  try {
    signal.throwIfAborted()
    encoded()
    id = await operations.admit(input)
    if (signal.aborted) abort()
    controller.signal.throwIfAborted()
    assets = await operations.prepare(input, controller.signal)
    controller.signal.throwIfAborted()
    handle = operations.video(
      { plan: input.plan, ratio: input.ratio, assets: assets.assets },
      input.localSources,
      input.resolvePlayback,
      controller.signal,
      originals,
    )
    jobs.push(
      handle.result.then((track) => {
        video = track
        picture = 1
        encoded()
      }, stopSibling),
    )
    jobs.push(
      operations
        .audio(input.plan, input.ratio, originals, controller.signal, (done, total) => {
          sound = done / total
          encoded()
        })
        .then((track) => {
          audio = track
          sound = 1
          encoded()
        }, stopSibling),
    )
    jobs.push(
      (async () => {
        for await (const value of handle!.progress) {
          picture = value.completedFrames / value.totalFrames
          encoded()
        }
      })(),
    )
    await Promise.all(jobs)
    controller.signal.throwIfAborted()
    if (!video) throw new Error('Missing browser video')
    progress({ stage: 'storing', percent: CLIP_BROWSER_RENDER.encodeProgressPercent })
    const project = await operations.store(
      id,
      video,
      audio,
      input,
      controller.signal,
      (percent) => {
        if (!controller.signal.aborted)
          progress({
            stage: 'storing',
            percent:
              CLIP_BROWSER_RENDER.encodeProgressPercent +
              ((CLIP_BROWSER_RENDER.storedProgressPercent -
                CLIP_BROWSER_RENDER.encodeProgressPercent) *
                percent) /
                100,
          })
      },
    )
    return project
  } catch (error) {
    controller.abort()
    const cancelled = await cancelAttempt()?.catch(() => undefined)
    // Completion may have won just before the owner pressed cancel. Keep that
    // durable result and never claim an accepted cancellation undid it.
    if (signal.aborted && cancelled === false) return operations.refresh(input.projectId)
    throw error
  } finally {
    controller.abort()
    handle?.cancel()
    await Promise.allSettled(jobs)
    assets?.dispose()
    originals.dispose()
    if (video) video.chunks.length = 0
    if (audio) audio.chunks.length = 0
    signal.removeEventListener('abort', abort)
  }
}
