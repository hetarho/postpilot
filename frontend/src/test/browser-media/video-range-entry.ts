import { freezeBrowserComposition } from '@/entities/clip-preview'
import { renderBrowserVideo } from '@/features/render-clip-browser'
import type { BrowserMediaSourceAccess } from '@/shared/lib'

declare global {
  interface Window {
    videoRangeFixture: {
      cursor(access: BrowserMediaSourceAccess, id: string, rate: number): Promise<unknown>
      render(
        url: string,
        fingerprint: string,
        cancelAt?: number,
        multipleTracks?: boolean,
      ): Promise<unknown>
    }
  }
}
window.videoRangeFixture = {
  async cursor(access, id, rate) {
    const worker = new Worker(new URL('./video-range.worker.ts', import.meta.url), {
      type: 'module',
    })
    try {
      return await new Promise((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('Fixture timed out')), 30_000)
        worker.onmessage = (event) => {
          clearTimeout(timeout)
          resolve(event.data)
        }
        worker.onerror = (event) => {
          clearTimeout(timeout)
          reject(new Error(event.message))
        }
        worker.postMessage({ access, id, ratePermille: rate })
      })
    } finally {
      worker.terminate()
    }
  },
  async render(url, fingerprint, cancelAt, multipleTracks = false) {
    const cuts = (
      multipleTracks
        ? [
            {
              id: 'first-native',
              startMs: 0,
              endMs: 1000,
              transitionMs: 0,
              playbackRatePermille: 1000,
              focal: { x: 0.5, y: 0.5 },
            },
          ]
        : [
            {
              id: 'late',
              startMs: 5000,
              endMs: 6500,
              transitionMs: 0,
              playbackRatePermille: 1000,
              focal: { x: 1, y: 0 },
            },
            {
              id: 'early',
              startMs: 200,
              endMs: 1200,
              transitionMs: 200,
              playbackRatePermille: 750,
              focal: { x: 0, y: 1 },
            },
            {
              id: 'middle',
              startMs: 2100,
              endMs: 3100,
              transitionMs: 300,
              playbackRatePermille: 1250,
              focal: { x: 0.5, y: 0.5 },
            },
          ]
    ).map((cut) => ({ ...cut, sourceId: 'original', fingerprint, copies: [], volumePermille: 0 }))
    const plan = {
      durationMs: multipleTracks ? 1000 : 3133,
      cuts,
      nativeComposition: true,
      elements: [],
    }
    const snapshot = await freezeBrowserComposition({
      ownerId: 'fixture',
      projectId: 'fixture',
      projectRevision: 1,
      planRevision: 1,
      plan,
      ratio: 'vertical',
      design: { hideDisclosure: true },
      sources: [
        {
          sourceId: 'original',
          fingerprint,
          durationMs: multipleTracks ? 2000 : 30000,
          width: multipleTracks ? 64 : 320,
          height: multipleTracks ? 32 : 180,
          hasAudio: false,
          allowedRatePermille: [500, 750, 1000, 1250, 1500, 2000],
        },
      ],
    })
    const controller = new AbortController()
    const handle = renderBrowserVideo(
      { plan, snapshot, ratio: 'vertical', assets: [], collectMeasurements: true },
      [],
      async () => url,
      controller.signal,
    )
    let completed = 0
    const progress = (async () => {
      for await (const value of handle.progress) {
        completed = value.completedFrames
        if (cancelAt !== undefined && completed >= cancelAt) controller.abort()
      }
    })()
    try {
      const video = await handle.result
      await progress
      let decoded = 0,
        decodeFailure: unknown
      let firstPixel: number[] | undefined
      const decoder = new VideoDecoder({
        output: (frame) => {
          decoded++
          if (!firstPixel) {
            const canvas = new OffscreenCanvas(1080, 1920),
              context = canvas.getContext('2d')!
            context.drawImage(frame, 0, 0)
            firstPixel = Array.from(context.getImageData(540, 960, 1, 1).data)
          }
          frame.close()
        },
        error: (error) => {
          decodeFailure = error
        },
      })
      try {
        decoder.configure(video.decoderConfig)
        for (const chunk of video.chunks) decoder.decode(new EncodedVideoChunk(chunk))
        await decoder.flush()
        if (decodeFailure) throw decodeFailure
      } finally {
        decoder.close()
      }
      return {
        frameCount: video.frameCount,
        decodedFrames: decoded,
        firstPixel,
        completed,
        config: video.config,
        durationUs: video.durationUs,
        sourceResources: video.sourceResources,
        phases: video.measurements,
        qualified: false,
      }
    } catch (error) {
      await progress
      await new Promise((resolve) => setTimeout(resolve, 1200))
      return {
        error: error instanceof Error ? error.message : String(error),
        name: error instanceof Error ? error.name : '',
        completed,
        qualified: false,
      }
    }
  },
}
