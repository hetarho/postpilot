import { describe, expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { timelineCuts, type ClipEditPlan } from '@/entities/clip-plan'
import { CLIP_BROWSER_COMPOSITION } from '@/entities/clip-design/@x/clip-preview'
import {
  BrowserSnapshotEpoch,
  evaluateBrowserFrame,
  freezeBrowserComposition,
  readBrowserCompositionSnapshot,
  type BrowserCompositionInput,
} from './browser-composition'
import fixture from './cut-timeline.fixture.json'

function input(plan = clipTimelineFixture().plan): BrowserCompositionInput {
  return {
    ownerId: 'alice',
    projectId: 'project',
    projectRevision: 4,
    planRevision: 3,
    plan,
    ratio: 'vertical',
    design: { hideDisclosure: true },
    sources: [
      ...new Map(
        plan.cuts.map((cut) => [
          cut.sourceId,
          {
            sourceId: cut.sourceId,
            fingerprint: cut.fingerprint,
            durationMs: 20000,
            width: 1920,
            height: 1080,
            hasAudio: true,
            allowedRatePermille: [500, 750, 1000, 1250, 1500, 2000],
          },
        ]),
      ).values(),
    ],
  }
}
function narration(plan: ClipEditPlan) {
  plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 700,
    segments: [
      {
        id: 'spoken',
        text: 'exact speech',
        textRevision: 1,
        inputHash: 'input',
        startMs: 1000,
        endMs: 2000,
        speech: {
          assetId: 'speech',
          voiceId: 'voice',
          bindingDigest: 'binding',
          inputHash: 'input',
          settingsHash: 'settings',
          audioHash: 'audio',
          profileId: 'profile',
          profileRevision: 1,
          samples: 44100,
          sampleRate: 44100,
          channels: 2,
          timing: [{ text: 'exact speech', startMs: 0, endMs: 1000 }],
        },
      },
    ],
  }
}
describe('frozen browser composition', () => {
  it('preserves legacy design defaults and only the exact declared native badge', async () => {
    const request = input()
    request.design = {
      introPreset: '',
      outroPreset: '',
      captionStyles: [],
      captionPace: 'rapid',
      accent: 'coral',
      disclosure: 'ad',
      hideDisclosure: false,
    }
    const snapshot = await freezeBrowserComposition(request)
    expect(snapshot.design).toMatchObject({
      introPreset: 'b',
      outroPreset: 'e',
      captionStyles: ['bold'],
      captionPace: 'rapid',
      accent: 'coral',
    })
    expect(
      snapshot.components.filter((c) => c.element.role === 'badge').map((c) => c.element.text),
    ).toEqual(['정확한 고정 문구'])
    expect(snapshot.components.some((c) => c.instanceId === 'product/disclosure')).toBe(false)
    request.design.disclosure = undefined
    expect(
      (await freezeBrowserComposition(request)).components.some(
        (c) => c.instanceId === 'product/disclosure',
      ),
    ).toBe(false)
    request.plan.elements = request.plan.elements!.filter((e) => e.role !== 'badge')
    expect(
      (await freezeBrowserComposition(request)).components.some(
        (c) => c.element.role === 'badge' || c.element.role === 'disclosure',
      ),
    ).toBe(false)
    request.plan.nativeComposition = false
    request.plan.elements = undefined
    await expect(freezeBrowserComposition(request)).rejects.toThrow(
      'CLIP_SNAPSHOT_INVALID:disclosure',
    )
    request.design.disclosure = 'ad'
    expect((await freezeBrowserComposition(request)).components.at(-1)!.element.text).toBe('광고')
  })
  it('freezes native project pace/accent while retaining verbatim stored plan data and projected phrases', async () => {
    const request = input()
    request.plan.elements = [request.plan.elements![0]!]
    const caption = request.plan.elements[0]!
    caption.pace = 'rapid'
    caption.accent = 'blue'
    caption.phrases = [
      { text: 'first exact phrase', startMs: 123, endMs: 456 },
      { text: 'second exact phrase', startMs: 500, endMs: 900 },
    ]
    request.design = { hideDisclosure: true, captionPace: 'steady', accent: 'coral' }
    const steady = await freezeBrowserComposition(request)
    expect(steady.components).toHaveLength(1)
    expect(steady.components[0]!.element).toMatchObject({
      pace: 'steady',
      accent: 'coral',
      text: caption.text,
    })
    expect(steady.plan.elements![0]!).toMatchObject({ pace: 'rapid', accent: 'blue' })
    request.design.captionPace = 'rapid'
    caption.pace = 'steady'
    const rapid = await freezeBrowserComposition(request)
    expect(rapid.components.map((c) => [c.startMs, c.endMs])).toEqual([
      [123, 456],
      [500, 900],
    ])
    expect(
      evaluateBrowserFrame(rapid, 4).components.map((c) => [c.text, c.animationProgress]),
    ).toEqual([['first exact phrase', 0.5]])
    expect(rapid.components[0]!.element.accent).toBe('coral')
  })
  it('retains recorded native steady repair and deterministically derives only missing rapid windows', async () => {
    const request = input()
    request.plan.elements = [request.plan.elements![0]!]
    const caption = request.plan.elements[0]!
    caption.fallbackReason = 'steady_copy'
    caption.ownerStyle = 'word-pop'
    request.design = { hideDisclosure: true, captionPace: 'rapid', accent: 'lime' }
    const repaired = await freezeBrowserComposition(request)
    expect(repaired.components).toHaveLength(1)
    expect(repaired.components[0]!.element).toMatchObject({
      pace: 'steady',
      accent: 'lime',
      fallbackReason: 'steady_copy',
    })
    caption.fallbackReason = undefined
    caption.text = '짧은 첫 문구 그리고 다음 문구'
    const derived = await freezeBrowserComposition(request)
    expect(derived.components.length).toBeGreaterThan(1)
    expect(derived.components.every((c) => c.element.pace === 'rapid')).toBe(true)
    expect(derived.plan.elements![0]!.phrases).toBeUndefined()
    expect(await readBrowserCompositionSnapshot(JSON.parse(JSON.stringify(derived)))).toEqual(
      derived,
    )
    caption.startMs = 120
    caption.endMs = 300
    await expect(freezeBrowserComposition(request)).rejects.toThrow(
      'CLIP_SNAPSHOT_RAPID_READABILITY',
    )
  })
  it('refuses malformed plan/source resource shapes with a named domain error', async () => {
    for (const patch of [
      { cuts: [{ copies: undefined }] },
      { elements: [{ rows: undefined, text: 'exact text' }] },
      { narration: { segments: [{ speech: { timing: undefined } }] } },
    ]) {
      const request = input()
      Object.assign(request.plan, patch)
      await expect(freezeBrowserComposition(request)).rejects.toThrow('CLIP_SNAPSHOT_INVALID')
    }
    const request = input()
    Object.assign(request.sources[0]!, { allowedRatePermille: undefined })
    await expect(freezeBrowserComposition(request)).rejects.toThrow('CLIP_SNAPSHOT_INVALID')
  })
  it('detaches every nested edit, placement, speech and manifest field, with hashes for exact resources', async () => {
    const request = input()
    request.plan.elements![0]!.ownerPosition = { x: 100, y: 900 }
    request.plan.sourceAudio = request.sources.map((s) => ({
      sourceId: s.sourceId,
      fingerprint: s.fingerprint,
      retainOriginalAudio: false,
    }))
    narration(request.plan)
    const snapshot = await freezeBrowserComposition(request)
    const before = JSON.stringify(snapshot)
    request.plan.elements![0]!.ownerPosition.x = 800
    request.plan.elements![0]!.rows.push({ role: 'line', text: 'changed' })
    request.plan.narration!.segments[0]!.speech!.audioHash = 'changed'
    request.plan.cuts[0]!.focal = { x: 1, y: 1 }
    expect(JSON.stringify(snapshot)).toBe(before)
    expect(Object.isFrozen(snapshot.plan.narration!.segments[0]!.speech!.timing)).toBe(true)
    expect(snapshot.fonts).toHaveLength(5)
    expect(snapshot.fonts.every((f) => /^[a-f0-9]{64}$/u.test(f.sha256))).toBe(true)
    expect(snapshot.snapshotFingerprint).toMatch(/^[a-f0-9]{64}$/u)
    expect(snapshot.speechFingerprint).toMatch(/^[a-f0-9]{64}$/u)
    expect(evaluateBrowserFrame(snapshot, 0).footageLayers[0]!.retainOriginalAudio).toBe(false)
    expect(await readBrowserCompositionSnapshot(JSON.parse(before))).toEqual(snapshot)
    expect(CLIP_BROWSER_COMPOSITION.qualified).toBe(false)
  })
  it('hashes changed text, geometry, sound, speech, versions and owner identity independently', async () => {
    const request = input()
    narration(request.plan)
    const original = await freezeBrowserComposition(request)
    for (const mutate of [
      (x: BrowserCompositionInput) => {
        x.plan.elements![0]!.text += '!'
      },
      (x: BrowserCompositionInput) => {
        x.plan.elements![0]!.ownerSizePx = 60
      },
      (x: BrowserCompositionInput) => {
        x.plan.sourceVolumePermille = 200
      },
      (x: BrowserCompositionInput) => {
        x.plan.narration!.volumePermille = 100
      },
      (x: BrowserCompositionInput) => {
        x.plan.narration!.segments[0]!.speech!.audioHash += '!'
      },
      (x: BrowserCompositionInput) => {
        x.ownerId = 'bob'
      },
    ]) {
      const changed = structuredClone(request)
      mutate(changed)
      expect((await freezeBrowserComposition(changed)).snapshotFingerprint).not.toBe(
        original.snapshotFingerprint,
      )
    }
  })
  it.each(['url', 'signedUrl', 'html', 'css', 'shader', 'javascript', 'bitmap', 'png', 'unknown'])(
    'refuses non-domain %s fields before freezing',
    async (key) => {
      const request = input()
      Object.assign(request.plan, { [key]: key === 'bitmap' ? new Uint8Array(2) : 'runtime data' })
      await expect(freezeBrowserComposition(request)).rejects.toThrow(
        'CLIP_SNAPSHOT_NON_DOMAIN_DATA',
      )
    },
  )
  it('stores authored markup as inert text and refuses functions/getters/resource objects', async () => {
    const request = input()
    request.plan.elements![0]!.text = '<script>alert(1)</script> https://caption.example'
    expect((await freezeBrowserComposition(request)).plan.elements![0]!.text).toBe(
      request.plan.elements![0]!.text,
    )
    for (const item of [() => 1, new Blob(['bytes'])]) {
      Object.assign(request.plan, { elements: item })
      await expect(freezeBrowserComposition(request)).rejects.toThrow('CLIP_SNAPSHOT_INVALID')
    }
    const getter = vi.fn(() => 'value')
    Object.defineProperty(request, 'plan', { get: getter })
    await expect(freezeBrowserComposition(request)).rejects.toThrow('CLIP_SNAPSHOT_NON_DOMAIN_DATA')
    expect(getter).not.toHaveBeenCalled()
  })
  it('refuses forged source fingerprints, stale speech, unsupported rates and component identities', async () => {
    for (const mutate of [
      (r: BrowserCompositionInput) => {
        r.plan.cuts[0]!.fingerprint = 'f'.repeat(64)
      },
      (r: BrowserCompositionInput) => {
        r.plan.cuts[0]!.playbackRatePermille = 501
      },
      (r: BrowserCompositionInput) => {
        r.plan.elements![0]!.ownerStyle = 'untrusted-code'
      },
      (r: BrowserCompositionInput) => {
        narration(r.plan)
        r.plan.narration!.segments[0]!.speech!.voiceId = 'foreign'
      },
      (r: BrowserCompositionInput) => {
        r.plan.cuts[0]!.id = r.plan.cuts[1]!.id
      },
      (r: BrowserCompositionInput) => {
        r.plan.durationMs++
      },
    ]) {
      const request = input()
      mutate(request)
      await expect(freezeBrowserComposition(request)).rejects.toThrow('CLIP_SNAPSHOT_')
    }
  })
  it('refuses malformed/cross-version serialized snapshots and tampered derived geometry', async () => {
    const snapshot = await freezeBrowserComposition(input())
    for (const mutate of [
      (x: Record<string, unknown>) => {
        x.schemaVersion = 2
      },
      (x: Record<string, unknown>) => {
        ;(x.versions as Record<string, unknown>).fonts = 'other'
      },
      (x: Record<string, unknown>) => {
        ;(x.components as { firstFrame: number }[])[0]!.firstFrame++
      },
      (x: Record<string, unknown>) => {
        x.snapshotFingerprint = 'a'.repeat(64)
      },
    ]) {
      const data: Record<string, unknown> = JSON.parse(JSON.stringify(snapshot))
      mutate(data)
      await expect(readBrowserCompositionSnapshot(data)).rejects.toThrow('CLIP_SNAPSHOT_')
    }
    await expect(readBrowserCompositionSnapshot(null)).rejects.toThrow(
      'CLIP_SNAPSHOT_INCOMPATIBLE_VERSION',
    )
    await expect(readBrowserCompositionSnapshot({ schemaVersion: 1 })).rejects.toThrow(
      'CLIP_SNAPSHOT_INCOMPATIBLE_VERSION',
    )
  })
})

describe('one output frame evaluator', () => {
  it.each(['bold', 'keynote', 'film', 'word-pop', 'neon', 'ember'])(
    'keeps exact 123–456ms visibility for %s while retaining native ink origin',
    async (style) => {
      const request = input()
      const caption = request.plan.elements![0]!
      request.plan.elements = [caption]
      caption.ownerStyle = style
      caption.startMs = 123
      caption.endMs = 456
      const snapshot = await freezeBrowserComposition(request)
      expect(snapshot.components[0]!).toMatchObject({
        firstFrame: 3,
        visibleFirstFrame: 4,
        endFrame: 14,
      })
      expect(evaluateBrowserFrame(snapshot, 3).components).toEqual([])
      expect(evaluateBrowserFrame(snapshot, 4).components[0]!.progress).toBeCloseTo(0.1)
      expect(evaluateBrowserFrame(snapshot, 13).components[0]!.progress).toBe(1)
      expect(evaluateBrowserFrame(snapshot, 14).components).toEqual([])
    },
  )
  it('keeps one declared badge with its authored copy, position and exact interval', async () => {
    const request = input()
    const badge = request.plan.elements!.find((e) => e.role === 'badge')!
    request.plan.elements = [badge]
    badge.text = 'authored badge'
    badge.position = 'top'
    badge.align = 'left'
    badge.basis = 'output-start'
    badge.startMs = 123
    badge.endMs = 456
    request.design = { hideDisclosure: false, disclosure: 'ad' }
    const snapshot = await freezeBrowserComposition(request)
    expect(snapshot.components).toHaveLength(1)
    expect(snapshot.components[0]!.element).toMatchObject({
      text: 'authored badge',
      position: 'top',
      align: 'left',
    })
    expect(evaluateBrowserFrame(snapshot, 3).components).toEqual([])
    expect(evaluateBrowserFrame(snapshot, 4).components[0]!.text).toBe('authored badge')
    expect(evaluateBrowserFrame(snapshot, 14).components).toEqual([])
  })
  it.each(fixture)(
    'matches native timeline fixture $name across every first/transition/last frame',
    async (sample) => {
      const cuts = sample.cuts.map((cut, index) => ({
        ...cut,
        sourceId: `source-${index}`,
        fingerprint: 'a'.repeat(63) + index,
        copies: [],
        volumePermille: 1000,
      }))
      const plan: ClipEditPlan = { cuts, durationMs: timelineCuts({ cuts }).at(-1)!.endMs }
      const snapshot = await freezeBrowserComposition(input(plan))
      expect(snapshot.frameCount).toBe(sample.frames.length)
      sample.frames.forEach((expected, frame) => {
        const evaluated = evaluateBrowserFrame(snapshot, frame)
        expect(evaluated.footageLayers).toHaveLength(expected.length)
        expected.forEach((layer, index) => {
          const actual = evaluated.footageLayers[index]!
          expect(actual.cutInstanceId).toBe(cuts[layer.cut]!.id)
          expect(actual.legacySourceMs).toBe(layer.sourceMs)
          expect(actual.weight).toBeCloseTo(layer.weight, 5)
          expect(actual.sourceTimestampUs).toBeLessThan(actual.sourceEndUs)
        })
        expect(evaluated.timestampUs).toBe(Math.round((frame * 1_000_000) / 30))
        expect(evaluated.durationUs).toBe(
          Math.round(((frame + 1) * 1_000_000) / 30) - evaluated.timestampUs,
        )
      })
      expect(() => evaluateBrowserFrame(snapshot, snapshot.frameCount)).toThrow(
        'CLIP_SNAPSHOT_INVALID:frame',
      )
    },
  )
  it('keeps split instances independent and exposes precise source microseconds and crop', async () => {
    const request = input()
    request.plan.elements = []
    const first = request.plan.cuts[0]!
    request.plan.cuts[1] = {
      ...first,
      id: 'same-source-again',
      startMs: 2000,
      endMs: 6000,
      transitionMs: 200,
    }
    first.focal = { x: 1, y: 0 }
    first.playbackRatePermille = 500
    request.plan.durationMs = timelineCuts(request.plan).at(-1)!.endMs
    const snapshot = await freezeBrowserComposition(request)
    const evaluated = evaluateBrowserFrame(snapshot, 1)
    expect(evaluated.footageLayers[0]!.sourceTimestampUs).toBe(16667)
    expect(evaluated.footageLayers[0]!.crop.left).toBeLessThan(0)
    const transition = evaluateBrowserFrame(snapshot, 594)
    expect(transition.footageLayers.map((l) => l.cutInstanceId)).toEqual([
      first.id,
      'same-source-again',
    ])
    expect(transition.footageLayers.map((l) => l.fingerprint)).toEqual([
      first.fingerprint,
      first.fingerprint,
    ])
    expect(transition.footageLayers[1]!.cutLocalFrame).toBe(0)
  })
  it('uses native frame-quantized progress and exact rapid phrase intervals', async () => {
    const request = input()
    request.plan.elements = [request.plan.elements![0]!]
    const element = request.plan.elements[0]!
    element.ownerStyle = 'word-pop'
    element.startMs = 120
    element.endMs = 2000
    const snapshot = await freezeBrowserComposition(request)
    const component = snapshot.components[0]!
    expect(component.firstFrame).toBe(3)
    expect(component.visibleFirstFrame).toBe(4)
    expect(evaluateBrowserFrame(snapshot, 3).components).toEqual([])
    expect(evaluateBrowserFrame(snapshot, 4).components[0]!.progress).toBeCloseTo(1 / 56)
    expect(evaluateBrowserFrame(snapshot, 59).components[0]!.progress).toBe(1)
    expect(evaluateBrowserFrame(snapshot, 60).components).toEqual([])
    element.pace = 'rapid'
    request.design!.captionPace = 'rapid'
    element.phrases = [
      { text: 'exact first', startMs: 120, endMs: 567 },
      { text: 'exact second', startMs: 567, endMs: 1000 },
    ]
    const rapid = await freezeBrowserComposition(request)
    expect(evaluateBrowserFrame(rapid, 17).components.map((c) => c.text)).toEqual(['exact first'])
    expect(
      evaluateBrowserFrame(rapid, 18).components.map((c) => [
        c.text,
        c.animationProgress,
        c.phraseIndex,
      ]),
    ).toEqual([['exact second', 0.5, 1]])
  })
  it('fences old callbacks after seek, replacement and cancel, releasing stale resources', async () => {
    const snapshot = await freezeBrowserComposition(input())
    const epoch = new BrowserSnapshotEpoch()
    const old = epoch.begin(snapshot)
    const seek = epoch.seek()
    expect(old.signal.aborted).toBe(true)
    const publish = vi.fn(),
      release = vi.fn()
    expect(epoch.publish(old, publish, release)).toBe(false)
    expect(release).toHaveBeenCalledOnce()
    expect(publish).not.toHaveBeenCalled()
    expect(epoch.publish(seek, publish)).toBe(true)
    const next = epoch.begin(await freezeBrowserComposition(input()))
    expect(() => epoch.assertCurrent(seek)).toThrow('CLIP_SNAPSHOT_SUPERSEDED')
    epoch.cancel()
    expect(next.signal.aborted).toBe(true)
    expect(epoch.publish(next, publish)).toBe(false)
  })
})
