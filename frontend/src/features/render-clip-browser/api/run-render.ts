import { type ClipEditPlan } from '@/entities/clip-plan'
import {
  clipRenderNeedsAudio,
  type BrowserVideoTrack,
  type ClipRenderCalls,
} from '@/entities/clip-preview'
import { type ClipProject, type ClipRatio } from '@/entities/clip-project'
import { CLIP_BROWSER_RENDER } from '@/entities/clip-design'
import { BrowserOriginals } from '../lib/originals'
import { prepareBrowserRenderAssets, type PreviewRequestCall } from './prepare-assets'
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
/** The run's collaborators, built from the entity call surfaces the feature was handed: the
 *  descriptors and the proto mapping stay in `entities/clip-*` (ARCH-17), and this feature owns
 *  what only a browser can do — encode, mux and put the bytes. */
export function browserRenderOperations(calls: {
  render: ClipRenderCalls
  fetchProject: (projectId: string) => Promise<ClipProject>
  requestPreview: PreviewRequestCall
}): BrowserRenderOperations {
  return {
    async admit(input) {
      const { renderId } = await calls.render.admit({
        projectId: input.projectId,
        expectedRevision: input.revision,
        batchId: input.batchId,
        machine: 'browser',
      })
      if (!renderId) throw new Error('Missing browser render identity')
      return renderId
    },
    cancel: (renderId) => calls.render.cancelBrowserRender(renderId),
    refresh: (id) => calls.fetchProject(id),
    prepare: (input, signal) =>
      prepareBrowserRenderAssets(
        calls.requestPreview,
        input.projectId,
        input.revision,
        input.plan,
        signal,
      ),
    video: renderBrowserVideo,
    audio: renderBrowserAudio,
    store: (id, video, audio, input, signal, progress) =>
      storeBrowserResult(
        id,
        video,
        audio,
        input.ratio,
        input.plan.durationMs,
        createBrowserResultStore(calls.render),
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
