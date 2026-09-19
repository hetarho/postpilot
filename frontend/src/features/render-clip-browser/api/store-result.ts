import { type BrowserVideoTrack, type ClipRenderCalls } from '@/entities/clip-preview'
import { type ClipProject, type ClipRatio } from '@/entities/clip-project'
import { muxMp4, type EncodedAudioTrack } from '@/shared/lib/media'
import { putBlobWithProgress } from '@/shared/lib/upload'
import { browserRenderVerdict, type BrowserRenderVerdict } from '../model/verdict'

export class BrowserRenderVerdictError extends Error {
  constructor(readonly notices: { code: string; action: string }[]) {
    super('Browser output failed validation')
    this.name = 'BrowserRenderVerdictError'
  }
}
export interface BrowserResultStore {
  prepare(
    renderId: string,
    bytes: number,
    signal: AbortSignal,
  ): Promise<{ putUrl: string; headers: Record<string, string> }>
  put: typeof putBlobWithProgress
  report(
    renderId: string,
    verdict: BrowserRenderVerdict,
    signal: AbortSignal,
  ): Promise<{ passed: boolean; notices: { code: string; action: string }[] }>
  complete(renderId: string, signal: AbortSignal): Promise<ClipProject>
}
export function createBrowserResultStore(calls: ClipRenderCalls): BrowserResultStore {
  return {
    prepare: (renderId, bytes, signal) => calls.prepareUpload(renderId, bytes, signal),
    put: putBlobWithProgress,
    report: (renderId, verdict, signal) => calls.reportVerdict(renderId, verdict, signal),
    complete: (renderId, signal) => calls.completeUpload(renderId, signal),
  }
}

/** Consumes the tracks and returns only the durable project. The caller must
 * never treat the local MP4 or a successful PUT as the project's result. */
export async function storeBrowserResult(
  renderId: string,
  video: BrowserVideoTrack,
  audio: EncodedAudioTrack | undefined,
  ratio: ClipRatio,
  durationMS: number,
  store: BrowserResultStore,
  signal: AbortSignal,
  progress: (percent: number) => void = () => {},
  mux: typeof muxMp4 = muxMp4,
): Promise<ClipProject> {
  let file: Blob | undefined
  try {
    signal.throwIfAborted()
    file = await mux(video, audio, signal)
    const verdict = browserRenderVerdict(video, audio, ratio, durationMS)
    video.chunks.length = 0
    if (audio) audio.chunks.length = 0
    signal.throwIfAborted()
    if (verdict.passed) {
      const upload = await store.prepare(renderId, file.size, signal)
      await store.put(upload.putUrl, upload.headers, file, progress, signal)
      signal.throwIfAborted()
    }
    file = undefined
    const report = await store.report(renderId, verdict, signal)
    if (!report.passed) throw new BrowserRenderVerdictError(report.notices)
    return await store.complete(renderId, signal)
  } finally {
    video.chunks.length = 0
    if (audio) audio.chunks.length = 0
  }
}
