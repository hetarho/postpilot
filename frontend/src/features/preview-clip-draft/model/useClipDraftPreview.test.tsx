import { webcrypto } from 'node:crypto'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { ClipEditPlan } from '@/entities/clip-plan'
import { useClipDraftPreview, type ClipDraftPreviewInput } from './useClipDraftPreview'

const plan = (): ClipEditPlan => ({
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
      copies: [],
      volumePermille: 1000,
      playbackRatePermille: 1000,
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
})
const input = (): ClipDraftPreviewInput => ({
  ownerId: 'owner',
  projectId: 'project',
  revision: 1,
  ratio: 'vertical',
  plan: plan(),
  sources: [
    {
      id: 'source',
      fingerprint: 'a'.repeat(64),
      filename: 'source.mp4',
      durationMs: 15000,
      width: 1920,
      height: 1080,
      hasAudio: false,
      allowedRatePermille: [1000],
    },
  ],
  resolvePlayback: vi.fn(async () => 'https://owned.test/source'),
  design: { hideDisclosure: true },
})
beforeEach(() => vi.stubGlobal('crypto', webcrypto))
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})
it('freezes current editing locally without raster or original access', async () => {
  const data = input(),
    view = renderHook(() => useClipDraftPreview(data))
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  expect(view.result.current.local!.snapshot.purpose).toBe('preview')
  expect(Object.isFrozen(view.result.current.local!.snapshot.plan)).toBe(true)
  expect(view.result.current.assets).toEqual([])
  expect(data.resolvePlayback).not.toHaveBeenCalled()
})
it('keeps its renderer identity through playhead changes and fences the old revision immediately', async () => {
  const data = input(),
    view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  const first = view.result.current.local
  view.rerender({ ...data, timeMs: 5000 })
  expect(view.result.current.local).toBe(first)
  view.rerender({ ...data, revision: 2 })
  expect(view.result.current.ready).toBe(false)
  expect(view.result.current.local).toBeUndefined()
  await waitFor(() => expect(view.result.current.local!.snapshot.planRevision).toBe(2))
})
it('preserves missing requested speech and an owner style outside the AI set', async () => {
  const data = input()
  data.plan.elements![0]!.ownerStyle = 'ember'
  data.plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 1000,
    segments: [
      {
        id: 'speech',
        text: '현재 장면',
        textRevision: 1,
        inputHash: 'input',
        startMs: 1000,
        endMs: 3000,
      },
    ],
  }
  const view = renderHook(() => useClipDraftPreview(data))
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  expect(view.result.current.local!.snapshot.components[0]!.componentId).toBe('caption/ember')
  expect(view.result.current.local!.snapshot.plan.narration!.segments[0]!.speech).toBeUndefined()
  expect(view.result.current.local!.snapshot.speechFingerprint).toBe('')
})
it('names an incompatible original and retries only on explicit refresh', async () => {
  const data = input()
  data.sources[0]!.fingerprint = 'b'.repeat(64)
  const view = renderHook(() => useClipDraftPreview(data))
  await waitFor(() => expect(view.result.current.failure).toBeDefined())
  expect(view.result.current.ready).toBe(false)
  act(() => view.result.current.onRetry())
  await waitFor(() => expect(view.result.current.failure).toBeDefined())
  expect(data.resolvePlayback).not.toHaveBeenCalled()
})
