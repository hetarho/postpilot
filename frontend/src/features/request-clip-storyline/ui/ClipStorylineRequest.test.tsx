import { useState } from 'react'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { CLIP_PROJECT_LIMITS, emptyClipProject, type ClipProject } from '@/entities/clip-project'
import type { GenerationJob } from '@/entities/generation-job'
import { ClipGenerationService, PlanService } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { ClipStorylineRequest } from './ClipStorylineRequest'

const project: ClipProject = {
  ...emptyClipProject(),
  id: 'clip',
  createdAt: '',
  updatedAt: '',
  editPlanRevision: 1,
  renderedPlanRevision: 0,
  storyline: {
    paragraphs: [{ text: '첫 장면', observationIds: [] }],
    editedByHand: false,
    addedSourceIds: [],
    takenOutObservationIds: [],
  },
}

// Isolated feature: three in-memory RPC handlers, no route tree, media or real services.
function setup(failure?: 'flush' | 'start', expired = false) {
  const quotes: Array<{ request: string; quoteId: string }> = []
  const starts: Array<{ request: string; quoteId: string }> = []
  const order: string[] = []
  const flush = vi.fn(async () => {
    order.push('flush')
    if (failure === 'flush') throw new Error('save refused')
  })
  const transport = createRouterTransport((router) => {
    router.rpc(PlanService.method.getMyPlan, () => ({ balance: { unlimited: true } }))
    router.rpc(ClipGenerationService.method.quoteClipStorylineRevision, (input) => {
      const quoteId = `quote-${quotes.length + 1}`
      quotes.push({ request: input.request, quoteId })
      return {
        quoteId,
        maxCredits: 4,
        expiresAt: expired ? '2000-01-01T00:00:00Z' : '2099-01-01T00:00:00Z',
        cancellationPolicy: {
          version: 1,
          unusedReservationNumerator: 1,
          unusedReservationDenominator: 2,
          rounding: 'ceil',
        },
      }
    })
    router.rpc(ClipGenerationService.method.startClipStorylineRevision, (input) => {
      order.push('start')
      starts.push(input)
      if (failure === 'start') throw new ConnectError('offline', Code.Unavailable)
      return { jobId: 'storyline-job' }
    })
  })
  function Composer({ show = true, job }: { show?: boolean; job?: GenerationJob }) {
    const [request, setRequest] = useState('')
    return (
      show && (
        <ClipStorylineRequest
          ownerId="alice"
          project={project}
          request={request}
          onRequestChange={setRequest}
          observe={{ providerId: 'p', modelId: 'observe' }}
          write={{ providerId: 'p', modelId: 'write' }}
          flush={flush}
          job={job}
        />
      )
    )
  }
  const view = render(<Composer />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  return {
    quotes,
    starts,
    order,
    flush,
    show: (show: boolean, job?: GenerationJob) => view.rerender(<Composer show={show} job={job} />),
    renew: () => {
      expired = false
    },
  }
}

const field = () => screen.getByLabelText('스토리라인 수정 요청')
async function openApproval() {
  await userEvent.click(screen.getByRole('button', { name: '스토리라인 수정 요청 보내기' }))
  const panel = within(await screen.findByRole('dialog', { name: '스토리라인 수정 요청 보내기' }))
  const approve = await panel.findByRole('button', { name: /승인하고 스토리라인 고치기/ })
  await waitFor(() => expect(approve).toBeEnabled())
  return approve
}

it('bounds controlled text by Unicode characters at the existing limit and counts it', () => {
  setup()
  fireEvent.change(field(), { target: { value: '🥗'.repeat(CLIP_PROJECT_LIMITS.instruction + 1) } })
  expect(field()).toHaveValue('🥗'.repeat(CLIP_PROJECT_LIMITS.instruction))
  expect(
    screen.getByText(`${CLIP_PROJECT_LIMITS.instruction} / ${CLIP_PROJECT_LIMITS.instruction}자`),
  ).toBeVisible()
  fireEvent.change(field(), { target: { value: ' \n ' } })
  expect(screen.getByRole('button', { name: '스토리라인 수정 요청 보내기' })).toBeDisabled()
})

it.each(['flush', 'start'] as const)(
  'retains failed %s text through remount and requires another explicit approval',
  async (failure) => {
    const fixture = setup(failure)
    const request = '첫 장면 🥗\n문구는  그대로'
    fireEvent.change(field(), { target: { value: request } })
    await userEvent.click(await openApproval())
    await waitFor(() => expect(fixture.flush).toHaveBeenCalledOnce())
    if (failure === 'start') await screen.findByRole('alert')
    expect(fixture.starts).toHaveLength(failure === 'start' ? 1 : 0)
    expect(field()).toHaveValue(request)
    const oldQuote = fixture.quotes.at(-1)!.quoteId
    fixture.show(false)
    fixture.show(true)
    expect(field()).toHaveValue(request)
    await waitFor(() => expect(fixture.quotes.at(-1)!.quoteId).not.toBe(oldQuote))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    const approve = await openApproval()
    expect(fixture.starts).toHaveLength(failure === 'start' ? 1 : 0)
    expect(approve).toBeEnabled()
  },
)

it('re-quotes retained words after remount, flushes before an explicit start and keeps submitted text', async () => {
  const fixture = setup()
  const request = '첫 장면 🥗\n문구는  그대로'
  fireEvent.change(field(), { target: { value: request } })
  await openApproval()
  const oldQuote = fixture.quotes.at(-1)!.quoteId
  fixture.show(false)
  fixture.show(true)
  expect(field()).toHaveValue(request)
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  const approve = await openApproval()
  const quote = fixture.quotes.at(-1)!
  expect(quote.request).toBe(request)
  expect(quote.quoteId).not.toBe(oldQuote)
  expect(fixture.starts).toEqual([])
  await userEvent.click(approve)
  await waitFor(() => expect(fixture.starts).toHaveLength(1))
  expect(fixture.starts[0]).toMatchObject(quote)
  expect(fixture.order).toEqual(['flush', 'start'])
  fixture.show(true, {
    id: 'storyline-job',
    kind: 'revise_storyline_clip',
    clipProjectId: 'clip',
    status: 'done',
    stage: 'storyline',
    progressDone: 1,
    progressTotal: 1,
    failure: undefined,
    postSlug: '',
    observeModel: undefined,
    writeModel: undefined,
    createdAt: '',
    updatedAt: '',
    targetLanguage: undefined,
  })
  expect(field()).toHaveValue(request)
})

it('requires a refreshed current quote and explicit approval after expiry', async () => {
  const fixture = setup(undefined, true)
  fireEvent.change(field(), { target: { value: '첫 장면부터' } })
  await userEvent.click(screen.getByRole('button', { name: '스토리라인 수정 요청 보내기' }))
  const panel = within(await screen.findByRole('dialog', { name: '스토리라인 수정 요청 보내기' }))
  const refresh = await panel.findByRole('button', { name: '최대 크레딧 다시 확인' })
  expect(panel.getByRole('button', { name: '스토리라인 수정 요청 보내기' })).toBeDisabled()
  expect(fixture.starts).toEqual([])
  fixture.renew()
  await userEvent.click(refresh)
  const approve = await panel.findByRole('button', { name: /승인하고 스토리라인 고치기/ })
  await waitFor(() => expect(approve).toBeEnabled())
  expect(fixture.starts).toEqual([])
  await userEvent.click(approve)
  await waitFor(() => expect(fixture.starts).toHaveLength(1))
  expect(fixture.starts[0]).toMatchObject(fixture.quotes.at(-1)!)
})
