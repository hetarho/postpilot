import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import type { GenerationJob } from '@/entities/generation-job'
import { voiceProfileQueryKey } from '@/entities/voice'
import { Stage } from '@/shared/api'
import { REVISION_INSTRUCTION_MAX_CHARS } from '@/shared/config'
import type { FakeRevisionStart } from '@/test/jobs'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { ReviseForm } from './ReviseForm'

const activeJob: GenerationJob = {
  id: 'active',
  kind: 'revise',
  status: 'running',
  stage: 'write',
  progressDone: 0,
  progressTotal: 1,
  error: '',
  postSlug: 'post',
  observeModel: undefined,
  writeModel: undefined,
  createdAt: '',
  updatedAt: '',
}

function renderForm({
  selected = true,
  active,
  revisions = [],
}: {
  selected?: boolean
  active?: GenerationJob
  revisions?: FakeRevisionStart[]
} = {}) {
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    providers: {
      models: [{ providerId: 'openrouter', modelId: 'writer' }],
      selections: selected
        ? [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }]
        : [],
    },
    jobs: { revisions, startJobId: 'revision-new' },
  })
  const onStarted = vi.fn()
  const queryClient = createTestQueryClient()
  render(<ReviseForm ownerId="alice" postSlug="post" voice={{ id: 'voice-a', deleted: false }} activeJob={active} onStarted={onStarted} />, {
    wrapper: withProviders(transport, queryClient),
  })
  return { onStarted, queryClient, transport }
}

it('requires an instruction and the explicit write selection', async () => {
  renderForm({ selected: false })

  expect(await screen.findByText('작성 모델을 선택하세요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '수정' })).toBeDisabled()
  expect(screen.getByLabelText('수정 요청')).toHaveAttribute(
    'maxlength',
    String(REVISION_INSTRUCTION_MAX_CHARS),
  )
})

it('stays disabled while another job is active', async () => {
  renderForm({ active: activeJob })

  expect(await screen.findByText('다른 작업이 진행 중이에요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '수정' })).toBeDisabled()
})

it('starts a revision with its instruction, rule flag, and selected write model', async () => {
  const revisions: FakeRevisionStart[] = []
  const { onStarted, queryClient, transport } = renderForm({ revisions })
  const ownProfile = voiceProfileQueryKey(transport, 'alice', 'voice-a')
  const siblingProfile = voiceProfileQueryKey(transport, 'alice', 'voice-b')
  queryClient.setQueryData(ownProfile, { profile: 'own' })
  queryClient.setQueryData(siblingProfile, { profile: 'sibling' })
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('수정 요청'), '  존댓말로  ')
  await user.click(screen.getByRole('checkbox', { name: '이 요청을 규칙으로 저장' }))
  const button = screen.getByRole('button', { name: '수정' })
  await waitFor(() => expect(button).toBeEnabled())

  await user.click(button)

  await waitFor(() => expect(onStarted).toHaveBeenCalledWith('revision-new'))
  expect(queryClient.getQueryState(ownProfile)?.isInvalidated).toBe(true)
  expect(queryClient.getQueryState(siblingProfile)?.isInvalidated).toBe(false)
  expect(revisions).toEqual([
    {
      postSlug: 'post',
      instruction: '존댓말로',
      saveAsRule: true,
      writeModel: { providerId: 'openrouter', modelId: 'writer' },
    },
  ])
})
