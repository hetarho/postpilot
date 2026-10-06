import { analysisGeometry, createAnalysisEncoder } from '@/features/prepare-clip-browser'
import type { AnalysisCopySlot, AnalysisSource } from '@/features/prepare-clip-browser'

declare global {
  interface Window {
    analysisFixture: {
      run(url: string, fingerprint: string, mode?: string): Promise<unknown>
    }
  }
}
const NativeWorker = Worker
/** Diagnostic-only decoder accounting around the unchanged production worker. */
window.Worker = class extends NativeWorker {
  constructor(url: string | URL, options?: WorkerOptions) {
    const bootstrap = new Blob(
      [
        `
      const frames = new Set(), samples = new Set(); let peakFrames = 0, peakAudio = 0;
      const frameClose = VideoFrame.prototype.close;
      VideoFrame.prototype.close = function() { frames.delete(this); return frameClose.call(this); };
      const audioClose = AudioData.prototype.close;
      AudioData.prototype.close = function() { samples.delete(this); return audioClose.call(this); };
      const NativeVideoDecoder = VideoDecoder, NativeAudioDecoder = AudioDecoder;
      self.VideoDecoder = class extends NativeVideoDecoder { constructor(options) { super({ ...options, output(frame) { frames.add(frame); peakFrames = Math.max(peakFrames, frames.size); options.output(frame); } }); } };
      self.AudioDecoder = class extends NativeAudioDecoder { constructor(options) { super({ ...options, output(sample) { samples.add(sample); peakAudio = Math.max(peakAudio, samples.size); options.output(sample); } }); } };
      const send = self.postMessage.bind(self);
      self.postMessage = (message, options) => { if (message.kind === 'result') message.result.resources = { peakDecodedFrames: peakFrames, liveDecodedFrames: frames.size, peakAudioData: peakAudio, liveAudioData: samples.size }; send(message, options); };
      await import(${JSON.stringify(String(url))});
    `,
      ],
      { type: 'text/javascript' },
    )
    const objectURL = URL.createObjectURL(bootstrap)
    super(objectURL, options)
    URL.revokeObjectURL(objectURL)
  }
}
window.analysisFixture = {
  async run(url, fingerprint, mode) {
    const controller = new AbortController(),
      encoder = createAnalysisEncoder(controller.signal)
    const source: AnalysisSource = {
      sourceId: 'synthetic',
      fingerprint,
      access: { kind: 'url', url },
    }
    const began = performance.now()
    try {
      const original = await encoder.measure(source)
      const measuredMs = performance.now() - began
      const copies = []
      for (let offset = 0, ordinal = 0; offset < original.durationMs; offset += 60000, ordinal++) {
        const slot: AnalysisCopySlot = {
          slot: `synthetic-${ordinal}`,
          sourceId: source.sourceId,
          fingerprint,
          ordinal,
          offsetMs: offset,
          durationMs: Math.min(60000, original.durationMs - offset),
          ...analysisGeometry(original.width, original.height),
          hasAudio: original.hasAudio,
          state: 'expected',
          bytes: 0,
          sha256: '',
        }
        if (mode === 'cancel') {
          controller.abort()
          await encoder.encode(source, slot)
          throw new Error('Cancellation failed')
        }
        const started = performance.now(),
          artifact = await encoder.encode(source, slot)
        const response = await fetch(`/__analysis__/output/${mode ?? 'source'}-${ordinal}`, {
          method: 'PUT',
          body: artifact.buffer,
        })
        if (!response.ok) throw new Error('Fixture save failed')
        copies.push({
          slot,
          sha256: artifact.sha256,
          inspection: artifact.inspection,
          encodedMs: performance.now() - started,
          resources: (artifact as typeof artifact & { resources: unknown }).resources,
        })
      }
      return {
        original,
        measuredMs,
        copies,
        originalResources: (original as typeof original & { resources: unknown }).resources,
        qualification: false,
      }
    } catch (error) {
      return {
        error: error instanceof Error ? error.message : String(error),
        name: error instanceof Error ? error.name : '',
        qualification: false,
      }
    } finally {
      encoder.close()
    }
  },
}
