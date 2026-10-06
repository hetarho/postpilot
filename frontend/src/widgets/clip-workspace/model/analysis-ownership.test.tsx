import { useSyncExternalStore } from 'react'
import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  ClipSourceSession,
  type SourcePipeline,
} from '@/features/upload-clip-sources/model/session'
import { emptyClipProject, type ClipProject, type ReadyClipBatch } from '@/entities/clip-project'
import type { AnalysisPreparationRequest } from '@/features/prepare-clip-browser'
import { useClipWorkspace } from './useClipWorkspace'

const state = vi.hoisted(() => ({
  session: undefined as unknown,
  signal: undefined as AbortSignal | undefined,
  gate: undefined as (() => void) | undefined,
}))
vi.mock('@/entities/clip-project', async (original) => ({
  ...(await original<object>()),
  useClipAnalysisPreparationCalls: () => ({}),
  useClipLifecycleApi: () => ({ cancel: vi.fn() }),
  useClipSourceCalls: () => ({}),
  useReorderClipSources: () => vi.fn(),
}))
vi.mock('@/entities/plan', () => ({ useMyPlan: () => ({}), hasServerExportAccess: () => false }))
vi.mock('@/entities/clip-preview', () => ({
  clipRenderNeedsAudio: () => false,
  useClipBrowserRenderCapability: () => ({}),
}))
vi.mock('@/features/upload-clip-sources', () => ({
  useClipSourceUpload: () => {
    const session = state.session as ClipSourceSession
    const upload = useSyncExternalStore(session.subscribe, session.getSnapshot)
    return {
      ...upload,
      ensurePlayback: session.ensurePlayback,
      acceptSoundBatch: session.acceptSoundBatch,
      acceptSourceOrder: session.acceptSourceOrder,
      beginAttempt: (id: string) => session.beginAttempt(id),
      markOwned: (id: string, job: string) => session.markOwned(id, job),
      finishAttempt: (id: string, status: 'done' | 'failed' | 'cancelled') =>
        session.finishAttempt(id, status),
    }
  },
}))
vi.mock('@/features/generate-clip', () => ({
  useGenerateClip: () => ({ busy: false, starting: false }),
}))
vi.mock('@/features/edit-clip-project', () => ({
  useClipDraftSave: () => ({ flush: vi.fn(), failing: false }),
  discardClipDraftQueue: vi.fn(),
}))
vi.mock('@/features/correct-clip', () => ({
  useClipCorrection: () => ({ draft: { cuts: [], elements: [] }, revision: 1, pending: false }),
}))
vi.mock('@/features/edit-clip-storyline', () => ({
  flushClipStoryline: vi.fn(),
  discardClipStorylineQueue: vi.fn(),
}))
vi.mock('@/features/edit-clip-regions', () => ({
  useClipRegionsEditor: () => ({ status: {}, invalid: false }),
  discardClipRegionQueue: vi.fn(),
}))
vi.mock('@/features/finalize-clip', () => ({ useFinalizeClip: () => ({ busy: false }) }))
vi.mock('@/features/cancel-clip', () => ({ useCancelClip: () => ({}) }))
vi.mock('@/features/render-clip-browser', () => ({ useBrowserRender: () => ({ busy: false }) }))
vi.mock('@/features/bind-clip-source-item', () => ({ useClipSourceBinding: () => ({}) }))
vi.mock('@/features/prepare-clip-browser/model/prepare', () => ({
  prepareBrowserAnalysis: vi.fn(async (_input, _ports, signal) => {
    state.signal = signal
    await new Promise<void>((resolve) => {
      state.gate = resolve
      signal.addEventListener('abort', () => resolve(), { once: true })
    })
    signal.throwIfAborted()
    return { jobId: 'job' }
  }),
}))

async function selectedSession() {
  const metadata = {
    filename: 'source.mp4',
    bytes: 1,
    contentType: 'video/mp4',
    durationMs: 1000,
    width: 16,
    height: 16,
    fingerprint: 'a'.repeat(64),
  }
  const source = {
    id: 'source',
    state: 'pending' as const,
    actualBytes: 0,
    retainOriginalAudio: false,
    metadata,
  }
  const batch = {
    id: 'batch',
    projectId: 'project',
    state: 'uploading' as const,
    expiresAt: '2099-01-01',
    sources: [source],
  }
  const ready: ReadyClipBatch = {
    ...batch,
    state: 'ready',
    sources: [{ ...source, state: 'ready', actualBytes: 1 }],
  }
  const pipeline: SourcePipeline = {
    read: async () => [metadata],
    reserve: async () => ({
      batch,
      uploads: [
        {
          sourceId: 'source',
          putUrl: 'https://private.invalid',
          headers: { 'If-None-Match': '*' },
        },
      ],
    }),
    put: async () => {},
    confirm: async () => ready,
    discard: async () => {},
    createURL: () => 'blob:source',
    revokeURL: () => {},
  }
  const session = new ClipSourceSession('project', pipeline)
  session.activate()
  await session.select([new File(['x'], 'source.mp4', { type: 'video/mp4' })])
  state.session = session
  return { session, ready }
}
const project: ClipProject = {
  ...emptyClipProject(),
  id: 'project',
  title: 'project',
  editPlanRevision: 1,
  renderedPlanRevision: 0,
  createdAt: '',
  updatedAt: '',
}
afterEach(() => {
  state.signal = undefined
  state.gate = undefined
  state.session = undefined
})
describe('real workspace preparation selection ownership', () => {
  it('keeps the same authoritative selection through real ready→accepting→owned transitions', async () => {
    const { session, ready } = await selectedSession()
    const hook = renderHook(() => useClipWorkspace('owner', project))
    const input = {
      projectId: project.id,
      revision: 1,
      batch: ready,
      quote: {},
    } as AnalysisPreparationRequest
    let run!: Promise<unknown>
    try {
      await act(async () => {
        expect(hook.result.current.generation.ownership.begin(ready.id)).toBe(true)
        run = hook.result.current.generation.analysis.preparation.run(input, async () => ({
          jobId: 'job',
        }))
        void run.catch(() => {})
        await Promise.resolve()
      })
      expect(session.getSnapshot().phase).toBe('accepting')
      expect(session.getSnapshot().readyBatch).toBeUndefined()
      expect(state.signal?.aborted).toBe(false)
      act(() => hook.result.current.generation.ownership.owned(ready.id, 'job'))
      expect(session.getSnapshot().phase).toBe('owned')
      expect(session.getSnapshot().attempt?.batchId).toBe(ready.id)
      expect(state.signal?.aborted).toBe(false)
      await act(async () => {
        state.gate?.()
        await run
      })
    } finally {
      state.gate?.()
      await run?.catch(() => {})
      hook.unmount()
      session.dispose()
    }
  })
  it.each(['owner', 'project', 'revision', 'selection'] as const)(
    'cancels actual %s replacement and refuses late preparation publication',
    async (kind) => {
      const { session, ready } = await selectedSession()
      const hook = renderHook(({ owner, value }) => useClipWorkspace(owner, value), {
        initialProps: { owner: 'owner', value: project },
      })
      let run!: Promise<unknown>
      await act(async () => {
        run = hook.result.current.generation.analysis.preparation.run(
          {
            projectId: project.id,
            revision: 1,
            batch: ready,
            quote: {},
          } as AnalysisPreparationRequest,
          async () => ({ jobId: 'job' }),
        )
        await Promise.resolve()
      })
      const rejected = expect(run).rejects.toThrow()
      try {
        if (kind === 'selection')
          await act(async () => {
            await session.select([
              new File(['replacement'], 'replacement.mp4', { type: 'video/mp4' }),
            ])
          })
        else
          act(() =>
            hook.rerender({
              owner: kind === 'owner' ? 'other' : 'owner',
              value: {
                ...project,
                id: kind === 'project' ? 'other-project' : project.id,
                editPlanRevision: kind === 'revision' ? 2 : 1,
              },
            }),
          )
        await act(async () => {
          await rejected
        })
        expect(state.signal?.aborted).toBe(true)
      } finally {
        state.gate?.()
        await run?.catch(() => {})
        hook.unmount()
        session.dispose()
      }
    },
  )
})
