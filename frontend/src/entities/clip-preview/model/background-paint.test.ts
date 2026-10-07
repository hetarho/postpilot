import { expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { freezeBrowserComposition, evaluateBrowserFrame } from './browser-composition'
import { backgroundScrim, drawMeasuredBrowserComponents } from './background-paint'
import type { BrowserBackgroundEvidence, BrowserBackgroundMeasurement } from './background-sampling'
import type { BrowserLocalComponent } from './local-components'

it('draws each component scrim immediately before its ink in native overlay order', async () => {
  const plan = clipTimelineFixture().plan
  plan.elements = plan
    .elements!.filter((e) => e.role === 'caption')
    .map((e) => ({
      ...e,
      cutId: '',
      basis: 'whole',
      startMs: undefined,
      endMs: undefined,
      resolvedStartMs: 0,
      resolvedEndMs: plan.durationMs,
    }))
  const snapshot = await freezeBrowserComposition({
    ownerId: 'owner',
    projectId: 'project',
    projectRevision: 1,
    planRevision: 1,
    plan,
    ratio: 'vertical',
    design: { hideDisclosure: true },
    sources: plan.cuts.map((c) => ({
      sourceId: c.sourceId,
      fingerprint: c.fingerprint,
      durationMs: 20000,
      width: 1920,
      height: 1080,
      hasAudio: false,
      allowedRatePermille: [1000],
    })),
  })
  const frame = evaluateBrowserFrame(snapshot, 0)
  const events: string[] = []
  const resources = frame.components.map((state) => ({
    component: state,
    draw: () => events.push(state.component.instanceId),
  })) as unknown as BrowserLocalComponent[]
  const evidence = {
    snapshotFingerprint: snapshot.authoritativeFingerprint ?? snapshot.snapshotFingerprint,
    localSnapshotFingerprint: snapshot.snapshotFingerprint,
    measurements: frame.components
      .map((state) => ({
        instanceId: state.component.instanceId,
        phraseIndex: 0,
        scrim: true,
        geometry: {
          region: { x: 10, y: 1300, width: 20, height: 20 },
          anchor: 'bottom',
          plate: false,
          motion: { inMs: 0, outMs: 0, dy: 0 },
        },
      }))
      .reverse(),
  } as BrowserBackgroundEvidence
  const context = {
    globalAlpha: 1,
    save: vi.fn(),
    restore: vi.fn(),
    translate: vi.fn(),
    fillRect: () => events.push('scrim'),
    createLinearGradient: () => ({ addColorStop: vi.fn() }),
  } as unknown as OffscreenCanvasRenderingContext2D
  drawMeasuredBrowserComponents(context, snapshot, frame, resources, evidence)
  expect(events).toEqual([
    'scrim',
    frame.components[0]!.component.instanceId,
    'scrim',
    frame.components[1]!.component.instanceId,
  ])
})
it('does not invent a radial caption scrim for anchors the native sampler does not support', () => {
  const m = {
    scrim: true,
    geometry: { region: { x: 10, y: 10, width: 20, height: 20 }, anchor: 'center', plate: false },
  } as BrowserBackgroundMeasurement
  expect(backgroundScrim('vertical', m)).toBeUndefined()
  m.geometry.anchor = 'upper_mid'
  expect(backgroundScrim('vertical', m)?.kind).toBe('linear')
  m.geometry.plate = true
  expect(backgroundScrim('vertical', m)).toBeUndefined()
})
