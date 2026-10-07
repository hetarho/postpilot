import { describe, expect, it } from 'vitest'
import {
  freezeBrowserComposition,
  freezeBrowserPreviewComposition,
  readBrowserCompositionSnapshot,
  type BrowserCompositionInput,
} from './browser-composition'
import { coveredBrowserFootage, selectBrowserAnchor } from './composition-placement'

const input = (): BrowserCompositionInput => ({
  ownerId: 'owner',
  projectId: 'project',
  projectRevision: 1,
  planRevision: 1,
  ratio: 'vertical',
  design: { hideDisclosure: true },
  sources: [
    {
      sourceId: 'source',
      fingerprint: 'a'.repeat(64),
      durationMs: 15000,
      width: 1080,
      height: 1920,
      hasAudio: false,
      originalMeasurementProvenance: 'browser_client',
      allowedRatePermille: [1000],
    },
  ],
  plan: {
    nativeComposition: true,
    durationMs: 15000,
    cuts: [
      {
        id: 'cut',
        sourceId: 'source',
        fingerprint: 'a'.repeat(64),
        startMs: 0,
        endMs: 15000,
        transitionMs: 0,
        volumePermille: 0,
        playbackRatePermille: 1000,
        copies: [],
      },
    ],
    elements: [
      {
        instanceId: 'caption',
        elementId: 'caption',
        cutId: '',
        kind: 'fixed',
        role: 'caption',
        text: '현재 장면',
        rows: [],
        style: 'bold',
        position: 'auto',
        align: 'center',
        basis: 'output-start',
        startMs: 1000,
        endMs: 6000,
        resolvedStartMs: 1000,
        resolvedEndMs: 6000,
        pace: 'steady',
        accent: '',
        keyword: '',
        groupId: '',
        itemId: '',
      },
    ],
  },
})

describe('native placement and immutable layout inputs', () => {
  it('ranks full containment and subject coverage after hard guards without an invented safe veto', () => {
    const candidates = [
      {
        anchor: 'upper_mid',
        align: 'center',
        fits: true,
        authoredAlign: false,
        plate: { x: 100, y: 100, width: 200, height: 80 },
      },
      {
        anchor: 'lower_mid',
        align: 'center',
        fits: true,
        authoredAlign: false,
        plate: { x: 100, y: 600, width: 200, height: 80 },
      },
    ]
    expect(
      selectBrowserAnchor(candidates, { x: 0, y: 0, width: 0, height: 0 }, [], false, '', [
        { x: 0, y: 500, width: 400, height: 300 },
      ]),
    ).toBe(1)
    expect(
      selectBrowserAnchor(candidates, { x: 0, y: 0, width: 0, height: 0 }, [], false, '', [
        { x: 0, y: 0, width: 10, height: 10 },
      ]),
    ).toBe(0)
    expect(
      selectBrowserAnchor(candidates, candidates[1]!.plate, [], false, '', [
        { x: 0, y: 500, width: 400, height: 300 },
      ]),
    ).toBe(0)
    expect(selectBrowserAnchor(candidates, candidates[1]!.plate, [], true, '', [])).toBe(-1)
    expect(
      selectBrowserAnchor(candidates, candidates[1]!.plate, [candidates[0]!.plate], false, '', []),
    ).toBe(1)
  })
  it('binds layout source identity/provenance and hashes deeply detached geometry', async () => {
    const value = input()
    value.layoutObservations = [
      {
        sourceId: 'source',
        fingerprint: 'a'.repeat(64),
        durationMs: 15000,
        width: 1080,
        height: 1920,
        originalMeasurementProvenance: 'browser_client',
        segments: [
          {
            startMs: 0,
            endMs: 15000,
            scene: 'food',
            readableText: true,
            subject: { x: 0.2, y: 0.3, width: 0.4, height: 0.5 },
            captionSafe: [{ x: 0, y: 0, width: 1, height: 0.2 }],
          },
        ],
      },
    ]
    const snapshot = await freezeBrowserComposition(value)
    expect(coveredBrowserFootage(snapshot, snapshot.components[0]!)).toEqual({
      subject: { x: 216, y: 576, width: 432.0000000000001, height: 960 },
      readableText: true,
      captionSafe: [{ x: 0, y: 0, width: 1080, height: 384 }],
    })
    expect(Object.isFrozen(snapshot.layoutObservations[0]!.segments[0]!.subject)).toBe(true)
    const forged = JSON.parse(JSON.stringify(snapshot)) as BrowserCompositionInput
    forged.layoutObservations![0]!.width++
    await expect(readBrowserCompositionSnapshot(forged)).rejects.toMatchObject({
      code: 'CLIP_SNAPSHOT_INVALID',
    })
    const changed = structuredClone(value)
    changed.layoutObservations![0]!.segments[0]!.subject.x = 0.1
    expect((await freezeBrowserComposition(changed)).snapshotFingerprint).not.toBe(
      snapshot.snapshotFingerprint,
    )
  })
  it('keeps editing usable with missing requested speech while strict export refuses it', async () => {
    const value = input()
    value.plan.narration = {
      enabled: true,
      confirmedVoiceId: 'voice',
      bindingDigest: 'binding',
      volumePermille: 1000,
      segments: [
        {
          id: 'speech',
          text: '현재 장면',
          textRevision: 1,
          startMs: 1000,
          endMs: 3000,
          inputHash: 'input',
        },
      ],
    }
    await expect(freezeBrowserComposition(value)).rejects.toMatchObject({
      code: 'CLIP_SNAPSHOT_INVALID',
    })
    const preview = await freezeBrowserPreviewComposition(value)
    expect(preview.purpose).toBe('preview')
    expect(preview.plan.narration!.enabled).toBe(true)
    expect((await readBrowserCompositionSnapshot(preview)).snapshotFingerprint).toBe(
      preview.snapshotFingerprint,
    )
  })
})
