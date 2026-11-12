import { webcrypto } from 'node:crypto'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { ClipEditPlan } from '@/entities/clip-plan'
import { createFiniteMediaSource } from '@/shared/lib/media'
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

const retained = (generation: number, url = '', state = 'available/unexpired') => ({
  sourceId: 'source',
  fingerprint: 'a'.repeat(64),
  url,
  retainedAccess: { generation, state },
})
const source = (local: NonNullable<ReturnType<typeof useClipDraftPreview>['local']>) =>
  local.source('source', 'a'.repeat(64), new AbortController().signal)

it('keeps a pending original owner through authoritative first publication and identical metadata', async () => {
  const data = { ...input(), localSources: [retained(0)] }
  let finish!: (url: string) => void
  data.resolvePlayback = vi.fn(
    () =>
      new Promise<string>((resolve) => {
        finish = resolve
      }),
  )
  const view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  const first = view.result.current.local!
  const pending = source(first)
  view.rerender({ ...data, localSources: [retained(1, 'https://owned.test/first')] })
  expect(view.result.current.local).toBe(first)
  finish('https://owned.test/first')
  expect(await pending).toEqual({ kind: 'url', url: 'https://owned.test/first' })
  view.rerender({ ...data, localSources: [retained(1, 'https://owned.test/first')] })
  expect(view.result.current.local).toBe(first)
  expect(data.resolvePlayback).toHaveBeenCalledOnce()
})

it.each([0, 1])(
  'fences generation %i to2 and makes renewed finite reads use the newly authorized URL',
  async (generation) => {
    const data = {
      ...input(),
      localSources: [retained(generation, generation ? 'https://owned.test/old' : '')],
    }
    const resolve = vi.fn(async () => 'https://owned.test/old')
    data.resolvePlayback = resolve
    const view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    const old = view.result.current.local!
    expect(await source(old)).toEqual({ kind: 'url', url: 'https://owned.test/old' })
    resolve.mockResolvedValue('https://owned.test/renewed')
    view.rerender({ ...data, localSources: [retained(2, 'https://owned.test/renewed')] })
    expect(view.result.current.ready).toBe(false)
    await expect(source(old)).rejects.toMatchObject({ name: 'AbortError' })
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    const access = await source(view.result.current.local!)
    const fetcher = vi.fn(
      async (_url: unknown) =>
        new Response(new Uint8Array([7]), {
          status: 206,
          headers: { 'Content-Range': 'bytes 0-0/1', 'Content-Length': '1' },
        }),
    )
    const reader = createFiniteMediaSource(
      access,
      {
        maxFileBytes: 10,
        maxReadBytes: 10,
        maxReadTotalBytes: 10,
        maxCacheBytes: 10,
        timeoutMs: 1000,
      },
      new AbortController().signal,
      { fetch: fetcher },
    )
    expect(await reader.read(0, 1)).toEqual(new Uint8Array([7]))
    expect(fetcher.mock.calls.every(([url]) => url === 'https://owned.test/renewed')).toBe(true)
    reader.dispose()
  },
)

it.each([undefined, -1, 0, 1.5, Number.NaN, Number.MAX_SAFE_INTEGER + 1])(
  'keeps conservative raw URL fencing for invalid/inconsistent generation %s',
  async (generation) => {
    const data: ClipDraftPreviewInput = {
      ...input(),
      localSources: [
        {
          ...retained(0, 'https://owned.test/old'),
          retainedAccess: generation === undefined ? undefined : { generation, state: 'available' },
        },
      ],
    }
    const view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    const old = view.result.current.local!
    view.rerender({
      ...data,
      localSources: [{ ...data.localSources![0]!, url: 'https://owned.test/new' }],
    })
    expect(view.result.current.ready).toBe(false)
    await expect(source(old)).rejects.toMatchObject({ name: 'AbortError' })
  },
)

it.each(['missing', 'expired', 'retention-changed', 'error/withdrawn'])(
  'fences explicit retained authority state %s even when the URL stays identical',
  async (state) => {
    const data = { ...input(), localSources: [retained(1, 'https://owned.test/old')] }
    const view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    const old = view.result.current.local!
    view.rerender({ ...data, localSources: [retained(1, 'https://owned.test/old', state)] })
    expect(view.result.current.ready).toBe(false)
    await expect(source(old)).rejects.toMatchObject({ name: 'AbortError' })
  },
)

it('shares one explicit forced renewal across publication/rebuild and then uses ordinary resolution for later renewals', async () => {
  const data = { ...input(), localSources: [retained(1, 'https://owned.test/old')] }
  let finish!: (url: string) => void,
    url = 'https://owned.test/old'
  const resolve = vi.fn((_fp: string, refresh?: boolean) =>
    refresh
      ? new Promise<string>((done) => {
          finish = done
        })
      : Promise.resolve(url),
  )
  data.resolvePlayback = resolve
  const view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  await source(view.result.current.local!)
  act(() => view.result.current.onRetry())
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  const first = source(view.result.current.local!)
  const firstCancelled = expect(first).rejects.toMatchObject({ name: 'AbortError' })
  await waitFor(() => expect(resolve.mock.calls.filter(([, refresh]) => refresh)).toHaveLength(1))
  url = 'https://owned.test/new'
  view.rerender({ ...data, localSources: [retained(2, url)] })
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  const current = source(view.result.current.local!)
  finish(url)
  await firstCancelled
  expect(await current).toEqual({ kind: 'url', url })
  expect(resolve.mock.calls.filter(([, refresh]) => refresh)).toHaveLength(1)
  url = 'https://owned.test/latest'
  view.rerender({ ...data, localSources: [retained(3, url)] })
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  expect(await source(view.result.current.local!)).toEqual({ kind: 'url', url })
  expect(resolve.mock.calls.at(-1)).toEqual(['a'.repeat(64), false])
})

it('latches a failed explicit force across withdrawal/rebuild without a normal fallback request', async () => {
  const data = { ...input(), localSources: [retained(1, 'https://owned.test/old')] }
  let fail!: (error: unknown) => void
  const resolve = vi.fn((_fp: string, refresh?: boolean) =>
    refresh
      ? new Promise<string>((_done, reject) => {
          fail = reject
        })
      : Promise.resolve('https://owned.test/old'),
  )
  data.resolvePlayback = resolve
  const view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  act(() => view.result.current.onRetry())
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  const pending = source(view.result.current.local!)
  const rejected = expect(pending).rejects.toMatchObject({ code: 'CLIP_SOURCE_UNAVAILABLE' })
  await waitFor(() => expect(resolve).toHaveBeenCalledOnce())
  fail({ reason: 'unavailable' })
  await rejected
  view.rerender({ ...data, localSources: [retained(2, '', 'error/withdrawn')] })
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  await expect(source(view.result.current.local!)).rejects.toMatchObject({
    code: 'CLIP_SOURCE_UNAVAILABLE',
  })
  expect(resolve).toHaveBeenCalledOnce()
})

it('keeps a same-URL successful retry stable and gives a later explicit retry its own attempt', async () => {
  const data = { ...input(), localSources: [retained(1, 'https://owned.test/old')] }
  const resolve = vi.fn(async (_fp: string, _refresh?: boolean) => 'https://owned.test/old')
  data.resolvePlayback = resolve
  const view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  for (let retry = 1; retry <= 2; retry++) {
    act(() => view.result.current.onRetry())
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    const local = view.result.current.local!
    const [video, audio] = await Promise.all([
      source(local),
      local.audioSource('a'.repeat(64), new AbortController().signal),
    ])
    expect(video).toEqual({ kind: 'url', url: 'https://owned.test/old' })
    expect(audio).toEqual(video)
    view.rerender({ ...data, localSources: [retained(1, 'https://owned.test/old')] })
    expect(view.result.current.local).toBe(local)
    expect(resolve).toHaveBeenCalledTimes(retry)
    expect(resolve.mock.calls.at(-1)).toEqual(['a'.repeat(64), true])
  }
})

it.each(['owner', 'project', 'revision', 'resolver', 'source'] as const)(
  'does not inherit an explicit attempt after a committed %s replacement, including a round trip',
  async (change) => {
    const data = { ...input(), localSources: [retained(1, 'https://owned.test/old')] }
    const resolve = vi.fn(async (_fp: string, refresh?: boolean) => {
      if (refresh) throw { reason: 'unavailable' }
      return 'https://owned.test/old'
    })
    data.resolvePlayback = resolve
    const view = renderHook((props: ClipDraftPreviewInput) => useClipDraftPreview(props), {
      initialProps: data,
    })
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    act(() => view.result.current.onRetry())
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    await expect(source(view.result.current.local!)).rejects.toMatchObject({
      code: 'CLIP_SOURCE_UNAVAILABLE',
    })
    const changed = {
      ...data,
      ...(change === 'owner'
        ? { ownerId: 'other' }
        : change === 'project'
          ? { projectId: 'other' }
          : change === 'revision'
            ? { projectRevision: 2 }
            : change === 'resolver'
              ? { resolvePlayback: vi.fn(async () => 'https://owned.test/other') }
              : {
                  localSources: [{ ...retained(1, 'https://owned.test/old'), sourceId: 'other' }],
                }),
    }
    view.rerender(changed)
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    view.rerender(data)
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    expect(await source(view.result.current.local!)).toEqual({
      kind: 'url',
      url: 'https://owned.test/old',
    })
    expect(resolve.mock.calls.at(-1)).toEqual(['a'.repeat(64), false])
    expect(resolve.mock.calls.filter(([, refresh]) => refresh)).toHaveLength(1)
  },
)

it('fences exact File replacement, File-to-retained and blob replacement without carrying an old owner', async () => {
  const data: ClipDraftPreviewInput = {
    ...input(),
    localSources: [{ ...retained(1, 'blob:first'), file: new File(['first'], 'same.mp4') }],
  }
  const view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
  await waitFor(() => expect(view.result.current.ready).toBe(true))
  let previous = view.result.current.local!
  const replacement = new File(['replacement'], 'same.mp4')
  for (const local of [
    { ...retained(1, 'blob:first'), file: replacement },
    retained(1, 'https://owned.test/retained'),
    retained(1, 'blob:second'),
    retained(1, 'blob:third'),
  ]) {
    view.rerender({ ...data, localSources: [local] })
    expect(view.result.current.ready).toBe(false)
    await expect(source(previous)).rejects.toMatchObject({ name: 'AbortError' })
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    previous = view.result.current.local!
    const access = await source(previous)
    if ('file' in local) expect(access).toEqual({ kind: 'blob', blob: replacement })
    else
      expect(access).toEqual({
        kind: 'url',
        url: local.url.startsWith('blob:') ? local.url : 'https://owned.test/source',
      })
  }
})

it.each(['file', 'blob'] as const)(
  'keeps a matching local %s owner through remote lease/availability changes',
  async (kind) => {
    const local = {
      ...retained(1, 'blob:local'),
      ...(kind === 'file' ? { file: new File(['pixels'], 'local.mp4') } : {}),
    }
    const data = { ...input(), localSources: [local] }
    const view = renderHook((props) => useClipDraftPreview(props), { initialProps: data })
    await waitFor(() => expect(view.result.current.ready).toBe(true))
    const previous = view.result.current.local
    view.rerender({
      ...data,
      localSources: [
        { ...local, retainedAccess: { generation: 2, state: 'expired/missing/new-lease' } },
      ],
    })
    expect(view.result.current.local).toBe(previous)
    expect(await source(previous!)).toEqual(
      kind === 'file' ? { kind: 'blob', blob: local.file } : { kind: 'url', url: 'blob:local' },
    )
    expect(data.resolvePlayback).not.toHaveBeenCalled()
  },
)
