import { describe, expect, it, vi } from 'vitest'
import type { ClipAnalysisPreparation } from '@/entities/clip-project'
import { prepareBrowserAnalysis, type AnalysisPreparationPorts } from './prepare'
import { analysisGeometry, validateAnalysisArtifact, validateMissingSlots } from './coverage'
import type { AnalysisCopyArtifact, AnalysisPreparationRequest } from './types'

const fingerprint = 'a'.repeat(64),
  hash = 'b'.repeat(64)
const original = {
  provenance: 'browser_client' as const,
  sourceId: 's',
  fingerprint,
  durationMs: 61_000,
  width: 1920,
  height: 1080,
  frameRateNumerator: 30000,
  frameRateDenominator: 1001,
  cadenceVerified: false,
  decodedFrames: 1829,
  hasAudio: true,
  audioRate: 44100,
  audioChannels: 2,
}
const slot = {
  slot: 's-0',
  sourceId: 's',
  fingerprint,
  ordinal: 0,
  offsetMs: 0,
  durationMs: 60_000,
  width: 720,
  height: 404,
  hasAudio: true,
  state: 'expected' as const,
  bytes: 0,
  sha256: '',
}
const preparation: ClipAnalysisPreparation = {
  id: 'prep',
  projectId: 'p',
  batchId: 'batch',
  revision: 3,
  state: 'preparing',
  expiresAt: '2099-01-01T00:00:00Z',
  originalMeasurementProvenance: 'browser_client',
  profile: {
    version: 'clip-browser-analysis-v1',
    intervalMs: 60000,
    longEdge: 720,
    framesPerSecond: 15,
    maxCopyBytes: 8 * 1024 * 1024,
    videoCodec: 'h264',
    pixelFormat: 'yuv420p',
    audioCodec: 'aac',
    audioRate: 48000,
    audioChannels: 1,
    audioBitrate: 64000,
    qualified: false,
  },
  copies: [slot, { ...slot, slot: 's-1', ordinal: 1, offsetMs: 60000, durationMs: 1000 }],
  progress: 0,
  failure: '',
  jobId: '',
}
const request: AnalysisPreparationRequest = {
  projectId: 'p',
  revision: 3,
  batch: {
    id: 'batch',
    projectId: 'p',
    state: 'ready',
    expiresAt: '2099-01-01',
    sources: [
      {
        id: 's',
        state: 'ready',
        actualBytes: 100,
        retainOriginalAudio: false,
        metadata: {
          filename: 's.mp4',
          contentType: 'video/mp4',
          bytes: 100,
          durationMs: 61000,
          width: 1920,
          height: 1080,
          fingerprint,
        },
      },
    ],
  },
  quote: { quoteId: 'q', maxCredits: 5, calls: [], expiresAt: '2099-01-01', binding: 'binding' },
}
function artifact(durationMs = 60000, hasAudio = true): AnalysisCopyArtifact {
  return {
    buffer: new ArrayBuffer(100),
    sha256: hash,
    inspection: {
      bytes: 100,
      videoFrames: (durationMs * 15) / 1000,
      videoStartMs: 0,
      videoEndMs: durationMs,
      containerEndMs: durationMs,
      audioSamples: hasAudio ? durationMs * 48 : 0,
      audioStartMs: 0,
      audioEndMs: hasAudio ? durationMs : 0,
      width: 720,
      height: 404,
      rotation: 0,
      hasAudio,
      targetVideoBitrate: 900000,
      actualVideoBitrate: 800000,
      actualAudioBitrate: hasAudio ? 64000 : 0,
    },
  }
}
function harness(overrides: Partial<AnalysisPreparationPorts> = {}) {
  const events: string[] = []
  let live = 0,
    peak = 0
  const encoder = {
    measure: vi.fn(async () => original),
    encode: vi.fn(async (_source, copy) => {
      live++
      peak = Math.max(peak, live)
      events.push(`encode:${copy.slot}`)
      return artifact(copy.durationMs, copy.hasAudio)
    }),
    close: vi.fn(),
  }
  const ports: AnalysisPreparationPorts = {
    access: vi.fn(async () => ({ kind: 'blob' as const, blob: new Blob(['original']) })),
    encoder: () => encoder,
    begin: vi.fn(async () => {
      events.push('begin')
      return structuredClone(preparation)
    }),
    reserve: vi.fn(async (input) => ({
      slot: input.slot,
      url: 'https://private.invalid/put',
      headers: { 'If-None-Match': '*' },
      expiresAt: '2099-01-01',
    })),
    upload: vi.fn(async (_url, options) => {
      expect(options?.credentials).toBe('omit')
      events.push('put')
      live--
      return new Response('', { status: 200 })
    }),
    complete: vi.fn(async () => {
      events.push('complete')
      return { ...preparation, state: 'verifying' as const, jobId: 'job' }
    }),
    cancelParent: vi.fn(async () => {
      events.push('cancel-parent')
    }),
    cancel: vi.fn(async () => {
      events.push('cancel-session')
    }),
    ...overrides,
  }
  const parent = vi.fn(async () => {
    events.push('start-parent')
    return { jobId: 'job' }
  })
  return { ports, encoder, events, parent, peak: () => peak }
}
describe('page-owned browser analysis preparation', () => {
  it('measures original VFR/44.1k independently and activates its parent before each sequential private copy', async () => {
    const h = harness()
    expect(
      await prepareBrowserAnalysis(request, h.ports, new AbortController().signal, h.parent),
    ).toEqual({ jobId: 'job' })
    expect(h.events).toEqual([
      'begin',
      'start-parent',
      'encode:s-0',
      'put',
      'encode:s-1',
      'put',
      'complete',
    ])
    expect(h.ports.begin).toHaveBeenCalledWith(
      expect.objectContaining({
        originals: [original],
        profileVersion: 'clip-browser-analysis-v1',
      }),
      expect.any(AbortSignal),
    )
    expect(h.parent).toHaveBeenCalledExactlyOnceWith('prep', expect.any(AbortSignal))
    expect(h.peak()).toBe(1)
    expect(h.encoder.close).toHaveBeenCalled()
  })
  it('reuses server-qualified retained intervals without encoding their gaps', async () => {
    const retained = { ...preparation, copies: [preparation.copies[1]] }
    const h = harness({
      begin: vi.fn(async () => retained),
      complete: vi.fn(async () => ({ ...retained, state: 'accepted' as const, jobId: 'job' })),
    })
    await prepareBrowserAnalysis(request, h.ports, new AbortController().signal, h.parent)
    expect(h.encoder.encode).toHaveBeenCalledTimes(1)
    expect(h.encoder.encode.mock.calls[0][1].offsetMs).toBe(60000)
  })
  it.each(['ownership', 'upload', 'coverage', 'capability', 'expired'])(
    'terminates %s failure without native or paid replay',
    async (kind) => {
      const h = harness()
      if (kind === 'ownership')
        h.ports.reserve = vi.fn(async () => ({
          slot: 'other-owner',
          url: 'https://private.invalid',
          headers: { 'If-None-Match': '*' },
          expiresAt: '2099-01-01',
        }))
      if (kind === 'upload') h.ports.upload = vi.fn(async () => new Response('', { status: 403 }))
      if (kind === 'coverage') h.encoder.encode.mockImplementation(async () => artifact(59000))
      if (kind === 'capability')
        h.encoder.encode.mockRejectedValue(new Error('CLIP_ANALYSIS_ENCODER_UNSUPPORTED'))
      if (kind === 'expired')
        h.ports.now = () =>
          Date.parse('2099-01-01') - 1 + (h.events.includes('start-parent') ? 100 : 0)
      await expect(
        prepareBrowserAnalysis(request, h.ports, new AbortController().signal, h.parent),
      ).rejects.toThrow()
      expect(h.ports.complete).not.toHaveBeenCalled()
      expect(h.parent).toHaveBeenCalledTimes(1)
      expect(h.events.slice(-2)).toEqual(['cancel-parent', 'cancel-session'])
    },
  )
  it('cancels local work and then durable parent/session on reselection or page leave', async () => {
    const h = harness(),
      controller = new AbortController()
    h.encoder.encode.mockImplementation(async () => {
      controller.abort()
      return artifact()
    })
    await expect(
      prepareBrowserAnalysis(request, h.ports, controller.signal, h.parent),
    ).rejects.toThrow()
    expect(h.ports.reserve).not.toHaveBeenCalled()
    expect(h.events.slice(-2)).toEqual(['cancel-parent', 'cancel-session'])
  })
  it('does not replay or upload after an uncertain parent activation', async () => {
    const h = harness()
    h.parent.mockRejectedValue(new Error('lost response'))
    await expect(
      prepareBrowserAnalysis(request, h.ports, new AbortController().signal, h.parent),
    ).rejects.toThrow('lost response')
    expect(h.encoder.encode).not.toHaveBeenCalled()
    expect(h.ports.cancel).toHaveBeenCalledExactlyOnceWith('prep', expect.any(AbortSignal))
  })
  it('rejects source measurement ownership and long selection before a parent exists', async () => {
    const h = harness()
    h.encoder.measure.mockResolvedValue({ ...original, durationMs: 30 * 60000 + 1 })
    await expect(
      prepareBrowserAnalysis(request, h.ports, new AbortController().signal, h.parent),
    ).rejects.toThrow('CLIP_INPUT_TOO_LARGE')
    expect(h.ports.begin).not.toHaveBeenCalled()
    expect(h.parent).not.toHaveBeenCalled()
  })
})
describe('original interval and completed artifact coverage', () => {
  it('keeps upright rotation aspect, even dimensions, and avoids upscaling', () => {
    expect(analysisGeometry(1080, 1920)).toEqual({ width: 404, height: 720 })
    expect(analysisGeometry(63, 35)).toEqual({ width: 62, height: 34 })
  })
  it('refuses duplicated seams, wrong remainders and unrelated retained coverage', () => {
    expect(() =>
      validateMissingSlots({ ...preparation, copies: [slot, slot] }, [original]),
    ).toThrow()
    expect(() =>
      validateMissingSlots({ ...preparation, copies: [{ ...slot, durationMs: 59000 }] }, [
        original,
      ]),
    ).toThrow()
    expect(() =>
      validateMissingSlots({ ...preparation, copies: [{ ...slot, fingerprint: 'c'.repeat(64) }] }, [
        original,
      ]),
    ).toThrow()
  })
  it('accepts silent complete copies and refuses size-truncated, rotated or incomplete completed streams', () => {
    expect(() =>
      validateAnalysisArtifact({ ...slot, hasAudio: false }, artifact(60000, false)),
    ).not.toThrow()
    const wrong = artifact()
    wrong.inspection.rotation = 90
    expect(() => validateAnalysisArtifact(slot, wrong)).toThrow()
    const huge = artifact()
    huge.buffer = new ArrayBuffer(8 * 1024 * 1024 + 1)
    expect(() => validateAnalysisArtifact(slot, huge)).toThrow()
    const truncated = artifact()
    truncated.inspection.audioSamples--
    truncated.inspection.audioEndMs = 59000
    expect(() => validateAnalysisArtifact(slot, truncated)).toThrow()
  })
})
