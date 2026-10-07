import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { ClipEditingStateSchema } from '@/shared/api'
import { toClipEditingState } from '@/entities/clip-plan/@x/clip-preview'
import {
  freezeBrowserComposition,
  readBrowserCompositionSnapshot,
} from '../model/browser-composition'

it('freezes a real default-protobuf read without request-only/default-owner fields and preserves chosen geometry', async () => {
  const fingerprint = 'a'.repeat(64)
  const wire = create(ClipEditingStateSchema, {
    plan: {
      nativeComposition: true,
      durationMs: 15000,
      cuts: [
        {
          id: 'cut',
          sourceId: 'source',
          fingerprint,
          startMs: 0,
          endMs: 15000,
          transitionMs: 0,
          playbackRatePermille: 1000,
        },
      ],
      elements: [
        {
          instanceId: 'caption',
          elementId: 'caption',
          kind: 'fixed',
          role: 'caption',
          text: '정확한 한글',
          style: 'bold',
          position: 'auto',
          effectivePosition: 'lower_mid',
          align: 'center',
          basis: 'output-start',
          startMs: 1000,
          endMs: 4000,
          resolvedStartMs: 1000,
          resolvedEndMs: 4000,
        },
      ],
    },
  })
  const plan = toClipEditingState(wire).plan
  expect(plan.elements![0]!.ownerSizePx).toBeUndefined()
  expect(Object.hasOwn(plan.elements![0]!, 'creation')).toBe(false)
  const request = {
    ownerId: 'owner',
    projectId: 'project',
    projectRevision: 1,
    planRevision: 1,
    plan,
    ratio: 'vertical' as const,
    design: { hideDisclosure: true },
    sources: [
      {
        sourceId: 'source',
        fingerprint,
        durationMs: 15000,
        width: 1920,
        height: 1080,
        hasAudio: false,
        allowedRatePermille: [1000],
      },
    ],
  }
  const snapshot = await freezeBrowserComposition(request)
  expect(snapshot.components[0]!.element.effectivePosition).toBe('lower_mid')
  const changed = JSON.parse(JSON.stringify(snapshot))
  changed.plan.elements[0].effectivePosition = 'top'
  await expect(readBrowserCompositionSnapshot(changed)).rejects.toThrow(
    'CLIP_SNAPSHOT_INVALID:derived contract',
  )
  plan.elements![0]!.effectivePosition = 'top'
  expect(snapshot.components[0]!.element.effectivePosition).toBe('lower_mid')
})
