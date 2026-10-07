import type { BrowserCompositionSnapshot } from '@/entities/clip-preview/model/browser-composition'
import type { BrowserAudioTrack } from '@/features/render-clip-browser/api/render-audio'

export type Arm = 'canvas2d-shared' | 'pixi-scene-canvas2d-hybrid'
export interface Fixture {
  id: string
  ratio: 'vertical' | 'horizontal' | 'square'
  style: string
  narrated: boolean
}
export interface Manifest {
  version: number
  manifestSha256: string
  source: {
    path: string; bytes: number; sha256: string; durationMs: number
    width: number; height: number; hasAudio: boolean; provenance: string
  }
  speech: {
    path: string; bytes: number; sha256: string
    samples: number; sampleRate: number; channels: number
  }
  cases: Fixture[]
  sampleFrames: number[]
  limits: {
    outputBytes: number; samplePngBytes: number; artifactChunkBytes: number
    sampleCount: number; runTimeoutMs: number
  }
}
export interface RunRequest {
  arm: Arm
  temperature: 'cold' | 'warm'
  fixture: Fixture
  manifest: Manifest
  snapshot: BrowserCompositionSnapshot
  audio?: BrowserAudioTrack
  audioEncodedIdentity?: unknown
  sourceUrl: string
}
export class Phases {
  private values: Record<string, { samples: number; totalMs: number; minMs: number; maxMs: number }> = {}
  add(name: string, elapsed: number) {
    if (!Number.isFinite(elapsed) || elapsed < 0) throw Error('BENCHMARK_INVALID_PHASE')
    const before = this.values[name]
    this.values[name] = {
      samples: (before?.samples ?? 0) + 1,
      totalMs: (before?.totalMs ?? 0) + elapsed,
      minMs: Math.min(before?.minMs ?? elapsed, elapsed),
      maxMs: Math.max(before?.maxMs ?? elapsed, elapsed),
    }
  }
  async measure<T>(name: string, operation: () => Promise<T> | T): Promise<T> {
    const start = performance.now()
    try { return await operation() } finally { this.add(name, performance.now() - start) }
  }
  snapshot() { return structuredClone(this.values) }
}
export async function sha256(bytes: ArrayBuffer | Uint8Array<ArrayBuffer>) {
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
  return [...digest].map(value => value.toString(16).padStart(2, '0')).join('')
}
export const shaJson = (value: unknown) => sha256(new TextEncoder().encode(JSON.stringify(value)))
export const flags = {
  productionQualified: false, renderingQualified: false, hardwareQualified: false,
  humanReleaseReviewComplete: false, analysisQualified: false, voiceQualified: false,
  physicalMemoryQualified: false, timingQualified: false,
}
