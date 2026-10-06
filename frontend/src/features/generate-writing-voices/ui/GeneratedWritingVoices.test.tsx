import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it, vi } from 'vitest'
import i18next from 'i18next'
import {
  GenerationService,
  GetGenerationResponseSchema,
  ProviderService,
  GetSelectionsResponseSchema,
  ListModelsResponseSchema,
  Stage,
  WritingVoiceCandidateService as Candidates,
} from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { getSelectionsQueryKey } from '@/entities/model-catalog'
import { i18n as candidateI18n } from '@/entities/voice-candidate'
import { i18n as flowI18n } from '../config/i18n'
import { GeneratedWritingVoices } from './GeneratedWritingVoices'

beforeEach(() => {
  i18next.addResourceBundle('ko', 'voices', candidateI18n.ko, true, true)
  i18next.addResourceBundle('ko', 'voices', flowI18n.ko, true, true)
})
const rows = (prefix = '스타일') =>
  Array.from({ length: 8 }, (_, n) => ({
    id: `style-${n}`,
    name: `${prefix} ${n + 1}`,
    description: '편안하게 말을 건네는 느낌이에요.',
    sample: '따뜻한 차 한 잔을 마시는 가상의 이야기예요.',
  }))
const deferred = <T,>() => {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}
function fixture({
  previous = false,
  startGate,
  adoptGate,
  failStart = false,
  free = false,
  fastComplete = false,
}: {
  previous?: boolean
  startGate?: ReturnType<typeof deferred<void>>
  adoptGate?: ReturnType<typeof deferred<void>>
  failStart?: boolean
  free?: boolean
  fastComplete?: boolean
} = {}) {
  const calls: string[] = []
  const starts: Array<{ providerId: string; modelId: string } | undefined> = []
  const estimates: Array<{ providerId: string; modelId: string } | undefined> = []
  const adoptions: Array<{ jobId: string; candidateId: string; makeDefault: boolean }> = []
  let latest = create(
    Candidates.method.getLatestWritingVoiceCandidates.output,
    previous
      ? {
          jobId: 'previous',
          resultJobId: 'previous',
          candidates: rows(),
        }
      : {},
  )
  let status = previous ? 'done' : 'running'
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.listModels, () =>
      create(ListModelsResponseSchema, {
        models: ['writer', 'new-writer'].map((modelId) => ({
          ref: { providerId: 'p', modelId },
          label: modelId,
          stages: [Stage.WRITE],
          levels: [{ stage: Stage.WRITE, level: 'free' }],
          affordable: true,
        })),
      }),
    )
    rpc(ProviderService.method.getSelections, () =>
      create(GetSelectionsResponseSchema, {
        selections: [{ stage: Stage.WRITE, ref: { providerId: 'p', modelId: 'writer' } }],
      }),
    )
    rpc(Candidates.method.getLatestWritingVoiceCandidates, () => {
      calls.push('Latest')
      return latest
    })
    rpc(ProviderService.method.initializeDefaultSelections, () =>
      create(GetSelectionsResponseSchema, {
        selections: [{ stage: Stage.WRITE, ref: { providerId: 'p', modelId: 'writer' } }],
      }),
    )
    rpc(Candidates.method.estimateWritingVoiceCandidates, (request) => {
      calls.push('Estimate')
      estimates.push(
        request.writeModel
          ? { providerId: request.writeModel.providerId, modelId: request.writeModel.modelId }
          : undefined,
      )
      return create(Candidates.method.estimateWritingVoiceCandidates.output, {
        credits: free ? 0 : 12,
        free,
      })
    })
    rpc(Candidates.method.startWritingVoiceCandidates, async (request) => {
      calls.push('Start')
      starts.push(
        request.writeModel
          ? { providerId: request.writeModel.providerId, modelId: request.writeModel.modelId }
          : undefined,
      )
      await startGate?.promise
      if (failStart) throw new ConnectError('private provider cause', Code.Unavailable)
      latest = create(Candidates.method.getLatestWritingVoiceCandidates.output, {
        ...latest,
        jobId: 'new-job',
      })
      status = 'running'
      if (fastComplete) {
        latest = create(Candidates.method.getLatestWritingVoiceCandidates.output, {
          jobId: 'new-job',
          resultJobId: 'new-job',
          candidates: rows('새 스타일'),
        })
        status = 'done'
      }
      return create(Candidates.method.startWritingVoiceCandidates.output, { jobId: 'new-job' })
    })
    rpc(GenerationService.method.getGeneration, (request) => {
      calls.push('Poll')
      return create(GetGenerationResponseSchema, {
        job: {
          id: request.id,
          kind: 'generate_writing_voice_candidates',
          status,
        },
      })
    })
    rpc(Candidates.method.cancelWritingVoiceCandidates, () => {
      calls.push('Cancel')
      status = 'cancelled'
      return create(Candidates.method.cancelWritingVoiceCandidates.output, {})
    })
    rpc(Candidates.method.adoptWritingVoiceCandidate, async (request) => {
      calls.push('Adopt')
      adoptions.push({
        jobId: request.jobId,
        candidateId: request.candidateId,
        makeDefault: request.makeDefault,
      })
      await adoptGate?.promise
      return create(Candidates.method.adoptWritingVoiceCandidate.output, {
        voice: {
          id: 'confirmed-voice',
          name: '스타일 1',
          made: true,
          origin: 2,
          isDefault: true,
        },
      })
    })
  })
  const cache = createTestQueryClient()
  return {
    calls,
    starts,
    estimates,
    adoptions,
    transport,
    cache,
    wrapper: withProviders(transport, cache),
    finish: async () => {
      status = 'done'
      latest = create(Candidates.method.getLatestWritingVoiceCandidates.output, {
        jobId: 'new-job',
        resultJobId: 'new-job',
        candidates: rows('새 스타일'),
      })
      await cache.invalidateQueries()
    },
    changeOwner: () => {
      latest = create(Candidates.method.getLatestWritingVoiceCandidates.output, {})
      status = 'running'
    },
  }
}
async function openGeneration(
  user: ReturnType<typeof userEvent.setup>,
  name = '스타일 8개 만들어 보기',
) {
  const button = await screen.findByRole('button', { name })
  await waitFor(() => expect(button).toBeEnabled())
  await user.click(button)
  const dialog = await screen.findByRole('dialog', { name: '새로운 스타일 8개를 만들까요?' })
  await within(dialog).findByText('이번 생성의 예상 사용량은 12크레딧이에요.')
  return dialog
}

it('entry and reads never start AI; explicit confirmation freezes the displayed write ref', async () => {
  const backend = fixture()
  const user = userEvent.setup()
  render(<GeneratedWritingVoices ownerId="alice" onAdopted={vi.fn()} />, {
    wrapper: backend.wrapper,
  })
  await screen.findByRole('button', { name: '스타일 8개 만들어 보기' })
  await waitFor(() =>
    expect(screen.getByRole('button', { name: '스타일 8개 만들어 보기' })).toBeEnabled(),
  )
  expect(backend.calls).not.toContain('Start')
  expect(backend.calls).not.toContain('Estimate')
  const dialog = await openGeneration(user)
  act(() =>
    backend.cache.setQueryData(
      getSelectionsQueryKey(backend.transport),
      create(GetSelectionsResponseSchema, {
        selections: [{ stage: Stage.WRITE, ref: { providerId: 'p', modelId: 'new-writer' } }],
      }),
    ),
  )
  await user.click(within(dialog).getByRole('button', { name: '8개 만들기' }))
  await waitFor(() => expect(backend.calls).toContain('Start'))
  expect(backend.starts).toEqual([{ providerId: 'p', modelId: 'writer' }])
  expect(backend.estimates[0]?.modelId).toBe('writer')
})

it('shows exactly eight labelled fictional styles and adopts only an explicitly selected style', async () => {
  const backend = fixture({ previous: true })
  const adopted = vi.fn()
  const user = userEvent.setup()
  render(<GeneratedWritingVoices ownerId="alice" onAdopted={adopted} />, {
    wrapper: backend.wrapper,
  })
  expect(await screen.findAllByRole('article')).toHaveLength(8)
  expect(screen.getAllByText('AI가 만든 말투')).toHaveLength(8)
  expect(screen.getAllByText('가상의 상황으로 쓴 예시')).toHaveLength(8)
  expect(screen.getByRole('button', { name: '이 말투 사용하기' })).toBeDisabled()
  expect(backend.calls).not.toContain('Start')
  await user.click(screen.getAllByRole('button', { name: '이 스타일 고르기' })[0])
  expect(backend.calls).not.toContain('Adopt')
  await user.click(screen.getByRole('button', { name: '이 말투 사용하기' }))
  await waitFor(() => expect(adopted).toHaveBeenCalledOnce())
  expect(backend.adoptions).toEqual([
    { jobId: 'previous', candidateId: 'style-0', makeDefault: true },
  ])
  expect(adopted.mock.calls[0][0]).toMatchObject({
    id: 'confirmed-voice',
    made: true,
    origin: 'synthetic',
  })
  expect(backend.calls).not.toContain('Start')
})

it('blocks duplicate starts, reports paid busy state, and cancels only after explicit confirmation', async () => {
  const startGate = deferred<void>()
  const backend = fixture({ startGate })
  const busy = vi.fn()
  const user = userEvent.setup()
  render(<GeneratedWritingVoices ownerId="alice" onAdopted={vi.fn()} onBusyChange={busy} />, {
    wrapper: backend.wrapper,
  })
  const dialog = await openGeneration(user)
  await user.dblClick(within(dialog).getByRole('button', { name: '8개 만들기' }))
  expect(backend.starts).toHaveLength(1)
  expect(busy).toHaveBeenCalledWith(true)
  act(() => startGate.resolve())
  await user.click(await screen.findByRole('button', { name: '생성 중단하기' }))
  expect(backend.calls).not.toContain('Cancel')
  const cancel = await screen.findByRole('dialog', { name: '말투 생성을 중단할까요?' })
  expect(within(cancel).getByText(/확인된 사용량만 정산/)).toBeVisible()
  await user.dblClick(within(cancel).getByRole('button', { name: '생성 중단하기' }))
  await waitFor(() => expect(backend.calls.filter((call) => call === 'Cancel')).toHaveLength(1))
  await waitFor(() => expect(busy).toHaveBeenLastCalledWith(false))
})

it('retains a previous batch after a failed reroll without exposing provider prose', async () => {
  const backend = fixture({ previous: true, failStart: true })
  const user = userEvent.setup()
  render(<GeneratedWritingVoices ownerId="alice" onAdopted={vi.fn()} />, {
    wrapper: backend.wrapper,
  })
  await screen.findByRole('heading', { name: '스타일 1' })
  const dialog = await openGeneration(user, '다른 스타일 8개 만들기')
  await user.click(within(dialog).getByRole('button', { name: '8개 만들기' }))
  await screen.findByRole('alert')
  expect(screen.getAllByRole('article')).toHaveLength(8)
  expect(screen.getByRole('heading', { name: '스타일 1' })).toBeVisible()
  expect(screen.queryByText('private provider cause')).not.toBeInTheDocument()
  expect(screen.getAllByRole('button', { name: '이 스타일 고르기' })[0]).toBeEnabled()
})

it('clears the selected card when a completed reroll reuses the candidate ids', async () => {
  const backend = fixture({ previous: true })
  const user = userEvent.setup()
  render(<GeneratedWritingVoices ownerId="alice" onAdopted={vi.fn()} />, {
    wrapper: backend.wrapper,
  })
  await user.click((await screen.findAllByRole('button', { name: '이 스타일 고르기' }))[0])
  const dialog = await openGeneration(user, '다른 스타일 8개 만들기')
  await user.click(within(dialog).getByRole('button', { name: '8개 만들기' }))
  await waitFor(() => expect(backend.calls).toContain('Start'))
  await act(async () => backend.finish())
  await screen.findByRole('heading', { name: '새 스타일 1' })
  expect(screen.getAllByRole('article')).toHaveLength(8)
  expect(screen.queryByRole('button', { name: '선택한 스타일' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '이 말투 사용하기' })).toBeDisabled()
})

it('does not publish a confirmed adoption to an obsolete account', async () => {
  const adoptGate = deferred<void>()
  const backend = fixture({ previous: true, adoptGate })
  const adopted = vi.fn()
  const user = userEvent.setup()
  const view = render(<GeneratedWritingVoices ownerId="alice" onAdopted={adopted} />, {
    wrapper: backend.wrapper,
  })
  await user.click((await screen.findAllByRole('button', { name: '이 스타일 고르기' }))[0])
  await user.click(screen.getByRole('button', { name: '이 말투 사용하기' }))
  await waitFor(() => expect(backend.calls).toContain('Adopt'))
  backend.changeOwner()
  view.rerender(<GeneratedWritingVoices ownerId="bob" onAdopted={adopted} />)
  await act(async () => {
    adoptGate.resolve()
    await adoptGate.promise
  })
  await waitFor(() => expect(screen.queryAllByRole('article')).toHaveLength(0))
  expect(adopted).not.toHaveBeenCalled()
})

it('shows a free quote before an explicit start and recovers a batch completed before start acknowledgement', async () => {
  const backend = fixture({ previous: true, free: true, fastComplete: true })
  const user = userEvent.setup()
  const busy = vi.fn()
  render(<GeneratedWritingVoices ownerId="alice" onAdopted={vi.fn()} onBusyChange={busy} />, {
    wrapper: backend.wrapper,
  })
  await waitFor(() =>
    expect(screen.getByRole('button', { name: '다른 스타일 8개 만들기' })).toBeEnabled(),
  )
  await user.click(screen.getByRole('button', { name: '다른 스타일 8개 만들기' }))
  const dialog = await screen.findByRole('dialog', { name: '새로운 스타일 8개를 만들까요?' })
  await within(dialog).findByText('이번 생성은 무료예요.')
  expect(backend.calls).not.toContain('Start')
  await user.click(within(dialog).getByRole('button', { name: '8개 만들기' }))
  await screen.findByRole('heading', { name: '새 스타일 1' })
  await waitFor(() => expect(busy).toHaveBeenLastCalledWith(false))
  expect(backend.starts).toHaveLength(1)
  expect(screen.getAllByRole('article')).toHaveLength(8)
})
