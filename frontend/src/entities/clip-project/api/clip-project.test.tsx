import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { createRouterTransport } from '@connectrpc/connect'
import { hashKey, type QueryKey } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ClipGenerationService, ClipTemplateService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { createTestQueryClient, withProviders } from '@/test/session'
import type { ClipAccounting, ClipProject } from '../model/types'
import {
  CHARGED_CLIP_KINDS,
  clipDetailPollInterval,
  clipProjectsKey,
  useClipProject,
  useClipProjectMutations,
  useClipProjects,
} from './clip-project'

function project(kind: string, status: string, accounting?: Partial<ClipAccounting>): ClipProject {
  return {
    id: 'clip',
    title: '제주',
    videoTemplateId: 'template',
    ratio: 'vertical',
    targetDurationMs: 15000,
    disclosure: 'ad',
    hideDisclosure: false,
    createdAt: '2026-10-01T00:00:00Z',
    updatedAt: '2026-10-01T00:00:00Z',
    editPlanRevision: 1,
    renderedPlanRevision: 0,
    latestJob: {
      id: 'job',
      kind,
      status,
      stage: '',
      progressDone: 0,
      progressTotal: 1,
      failure: undefined,
      postSlug: '',
      clipProjectId: 'clip',
      observeModel: undefined,
      writeModel: undefined,
      createdAt: '2026-10-01T00:00:00Z',
      updatedAt: '2026-10-01T00:00:00Z',
      targetLanguage: undefined,
    },
    accounting: accounting
      ? { jobId: 'job', status: 'settling', settled: false, ...accounting }
      : undefined,
  }
}

describe('the detail poll', () => {
  it('mirrors chargedClipKind in backend/internal/clip/app/accounting.go', () => {
    const source = readFileSync(
      resolve(import.meta.dirname, '../../../../../backend/internal/clip/app/accounting.go'),
      'utf8',
    )
    const body = source.split('func chargedClipKind(kind string) bool {')[1]!.split('\n}')[0]!
    const server = [...body.matchAll(/"([a-z_]+)"/g)].map((match) => match[1]!)
    expect(server.length).toBeGreaterThan(0)
    expect([...CHARGED_CLIP_KINDS].sort()).toEqual(server.sort())
  })

  it.each(['storyline_clip', 'revise_storyline_clip'])(
    'stops once a %s job ends, with no accounting to wait for',
    (kind) => {
      expect(clipDetailPollInterval(project(kind, 'done'))).toBe(false)
      expect(clipDetailPollInterval(project(kind, 'failed'))).toBe(false)
      expect(clipDetailPollInterval(project(kind, 'running'))).toBe(POLL_INTERVAL_MS)
    },
  )

  it.each(['generate_clip', 'revise_clip', 'speech_clip'])(
    'keeps reading a finished %s until its settlement arrives',
    (kind) => {
      expect(POLL_INTERVAL_MS).toBe(2_000)
      expect(clipDetailPollInterval(project(kind, 'done'))).toBe(POLL_INTERVAL_MS)
      expect(clipDetailPollInterval(project(kind, 'done', { settled: false }))).toBe(
        POLL_INTERVAL_MS,
      )
      expect(clipDetailPollInterval(project(kind, 'done', { jobId: 'older', settled: true }))).toBe(
        POLL_INTERVAL_MS,
      )
      expect(
        clipDetailPollInterval(project(kind, 'done', { status: 'settled', settled: true })),
      ).toBe(false)
    },
  )

  it('polls nothing without a job', () => {
    expect(
      clipDetailPollInterval({ ...project('generate_clip', 'done'), latestJob: undefined }),
    ).toBe(false)
    expect(clipDetailPollInterval(undefined)).toBe(false)
  })
})

describe('a settings save', () => {
  const draft = {
    title: '성수',
    videoTemplateId: 'template',
    ratio: 'vertical' as const,
    targetDurationMs: 15000,
    disclosure: 'ad' as const,
    hideDisclosure: false,
  }
  it('persists the selected confirmed voice and disabled state through saves and reloads', async () => {
    let stored = {
      ...draft,
      id: 'clip',
      editPlanRevision: 0,
      dubbing: { enabled: true, voiceId: 'voice' },
    }
    const received: unknown[] = []
    const transport = createRouterTransport(({ rpc }) => {
      rpc(ClipGenerationService.method.updateClipProject, (request) => {
        received.push(request.dubbing)
        stored = {
          ...stored,
          dubbing: { enabled: request.dubbing!.enabled, voiceId: request.dubbing!.voiceId },
        }
        return { project: stored }
      })
      rpc(ClipGenerationService.method.getClipProject, () => ({ project: stored }))
    })
    const client = createTestQueryClient()
    const view = renderHook(
      () => ({
        detail: useClipProject('alice', 'clip'),
        mutations: useClipProjectMutations('alice'),
      }),
      { wrapper: withProviders(transport, client) },
    )
    await waitFor(() => expect(view.result.current.detail.data?.dubbing?.voiceId).toBe('voice'))
    for (const enabled of [true, false, true]) {
      await act(() =>
        view.result.current.mutations.save.mutateAsync({
          id: 'clip',
          draft: { ...draft, dubbing: { enabled, voiceId: 'voice' } },
        }),
      )
      await act(() => view.result.current.detail.refetch())
      expect(view.result.current.detail.data?.dubbing).toEqual({ enabled, voiceId: 'voice' })
    }
    expect(received).toHaveLength(3)
  })
  function backend(answer: { editPlanRevision: number }) {
    const calls: string[] = []
    let listReads = 0
    let releaseList: (() => void) | undefined
    const listHeld = new Promise<void>((resolve) => (releaseList = resolve))
    const stored = {
      id: 'clip',
      title: '제주',
      videoTemplateId: 'template',
      ratio: 'vertical',
      targetDurationMs: 15000,
      disclosure: 'ad',
      editPlanRevision: 1,
    }
    const transport = createRouterTransport(({ rpc }) => {
      rpc(ClipGenerationService.method.getClipProject, () => {
        calls.push('GetClipProject')
        // The detail read composes the job around the stored project; an update's answer does not.
        return {
          project: {
            ...stored,
            latestJob: { id: 'job', kind: 'generate_clip', status: 'done', clipProjectId: 'clip' },
            accounting: {
              jobId: 'job',
              status: 'settled',
              settled: true,
              finalChargeCredits: 3,
              refundCredits: 0,
            },
            canFinalize: true,
          },
        }
      })
      rpc(ClipGenerationService.method.listClipProjects, async () => {
        calls.push('ListClipProjects')
        // The refetch a save asks for is held: the save must not wait on it.
        if (++listReads > 1) await listHeld
        return { projects: [stored] }
      })
      rpc(ClipGenerationService.method.updateClipProject, (request) => {
        calls.push('UpdateClipProject')
        Object.assign(stored, { title: request.title ?? stored.title, ...answer })
        return { project: { ...stored, canFinalize: false } }
      })
      rpc(ClipTemplateService.method.listVideoTemplates, () => {
        calls.push('ListVideoTemplates')
        return { templates: [] }
      })
    })
    return { calls, transport, releaseList: () => releaseList?.() }
  }
  function mount(transport: ReturnType<typeof backend>['transport']) {
    const client = createTestQueryClient()
    const view = renderHook(
      () => ({
        detail: useClipProject('alice', 'clip'),
        list: useClipProjects('alice'),
        mutations: useClipProjectMutations('alice'),
      }),
      { wrapper: withProviders(transport, client) },
    )
    return { view, client }
  }
  const keysOf = (spy: { mock: { calls: unknown[][] } }) =>
    spy.mock.calls.map((call) => hashKey((call[0] as { queryKey: QueryKey }).queryKey))

  it('takes the answer into the detail, reads it no more, and leaves the list refetching', async () => {
    const server = backend({ editPlanRevision: 1 })
    const { view, client } = mount(server.transport)
    await waitFor(() => expect(view.result.current.detail.data?.title).toBe('제주'))
    await waitFor(() => expect(view.result.current.list.data).toHaveLength(1))
    const invalidate = vi.spyOn(client, 'invalidateQueries')

    await act(() =>
      view.result.current.mutations.save.mutateAsync({ id: 'clip', draft: { ...draft } }),
    )

    // Resolved while the list's refetch is still held, and without reading the detail again.
    expect(server.calls.filter((call) => call === 'ListClipProjects')).toHaveLength(2)
    expect(server.calls.filter((call) => call === 'GetClipProject')).toHaveLength(1)
    const key = clipProjectsKey(server.transport, 'alice')
    expect(keysOf(invalidate)).toEqual([
      hashKey([...key, 'list']),
      hashKey(['clip-templates', server.transport, 'alice']),
    ])
    const detail = client.getQueryData<ClipProject>([...key, 'detail', 'clip'])!
    expect(detail.title).toBe('성수')
    // What only the detail read carries stays as it was read.
    expect(detail.latestJob?.id).toBe('job')
    expect(detail.accounting?.settled).toBe(true)
    expect(detail.canFinalize).toBe(true)
    server.releaseList()
    await waitFor(() => expect(view.result.current.list.isFetching).toBe(false))
    expect(server.calls.filter((call) => call === 'GetClipProject')).toHaveLength(1)
  })

  it('reads the detail again when the save moved the plan, before the lane moves on', async () => {
    const server = backend({ editPlanRevision: 2 })
    const { view, client } = mount(server.transport)
    await waitFor(() => expect(view.result.current.detail.data?.title).toBe('제주'))

    await act(() =>
      view.result.current.mutations.save.mutateAsync({ id: 'clip', draft: { ...draft } }),
    )

    // Settled by the time the save resolves, so the lane's next write rebases onto it.
    expect(server.calls.filter((call) => call === 'GetClipProject')).toHaveLength(2)
    const detail = client.getQueryData<ClipProject>([
      ...clipProjectsKey(server.transport, 'alice'),
      'detail',
      'clip',
    ])!
    expect(detail.editPlanRevision).toBe(2)
    expect(detail.latestJob?.id).toBe('job')
    server.releaseList()
  })
})
