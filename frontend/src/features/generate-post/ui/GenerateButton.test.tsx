import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import { Stage } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeGenerationStart } from '@/test/jobs'
import { GenerateButton } from './GenerateButton'

const post: PostDraft = {
  slug: '20260829-jeju',
  title: '제주',
  memo: '비 온 뒤 산책',
  status: 'draft',
  createdAt: '2026-08-29T00:00:00Z',
  updatedAt: '',
  images: [
    {
      id: 'image-1',
      filename: 'IMG_1.jpg',
      width: 1024,
      height: 768,
      bytes: 200_000,
      viewUrl: 'https://storage.test/IMG_1.jpg?signature=read',
    },
  ],
  activeJob: undefined,
  content: undefined,
  observations: [],
  pendingExperimentId: '',
  contentRevision: 0n,
  machineBaselineRevision: 0n,
  canFinalize: false,
  targetLength: 1200,
}

const activeJob: GenerationJob = {
  id: 'job-active',
  kind: 'generate',
  status: 'running',
  stage: 'write',
  progressDone: 0,
  progressTotal: 1,
  error: '',
  postSlug: post.slug,
  observeModel: undefined,
  writeModel: undefined,
  createdAt: '',
  updatedAt: '',
}

function renderButton({
  starts = [],
  selected = true,
  active,
  draft = post,
}: {
  starts?: FakeGenerationStart[]
  selected?: boolean
  active?: GenerationJob
  draft?: PostDraft
} = {}) {
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    providers: {
      models: [
        { providerId: 'openrouter', modelId: 'vision', vision: true },
        { providerId: 'openrouter', modelId: 'writer' },
        { providerId: 'openrouter', modelId: 'writer-b' },
      ],
      selections: selected
        ? [
            { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'vision' },
            { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
          ]
        : [],
      comparisonPairs: selected
        ? [
            {
              stage: Stage.WRITE,
              candidateA: { providerId: 'openrouter', modelId: 'writer' },
              candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
            },
          ]
        : [],
    },
    jobs: { starts, startJobId: 'job-new' },
  })
  const onStarted = vi.fn()
  render(<GenerateButton post={draft} activeJob={active} onStarted={onStarted} />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  return { onStarted }
}

it('disables generation with the inline reason when the write choice is missing', async () => {
  renderButton({ selected: false })

  expect(await screen.findByText(/작성 A\/B 모델을 선택하세요/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
})

it('disables generation while the post has an active job', async () => {
  renderButton({ active: activeJob })

  expect(await screen.findByText('이미 생성 중이에요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
})

it('stays disabled while the accepted job is waiting for its first snapshot', async () => {
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    providers: {
      models: [
        { providerId: 'openrouter', modelId: 'vision', vision: true },
        { providerId: 'openrouter', modelId: 'writer' },
        { providerId: 'openrouter', modelId: 'writer-b' },
      ],
      selections: [
        { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'vision' },
        { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
      ],
      comparisonPairs: [
        {
          stage: Stage.WRITE,
          candidateA: { providerId: 'openrouter', modelId: 'writer' },
          candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
        },
      ],
    },
  })
  render(<GenerateButton post={post} jobPending onStarted={vi.fn()} />, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })

  expect(await screen.findByText('생성 작업을 확인하는 중이에요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
})

it('blocks a second generation until the pending experiment is reviewed', async () => {
  renderButton({ draft: { ...post, pendingExperimentId: 'experiment-1' } })

  expect(await screen.findByText(/먼저 대기 중인 AI 결과를 확인해 주세요/)).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
  expect(screen.getByRole('link', { name: 'AI 결과 확인' })).toHaveAttribute(
    'href',
    '/ai-models/experiments/experiment-1',
  )
})

it('starts with the selected observe and write model payload', async () => {
  const starts: FakeGenerationStart[] = []
  const { onStarted } = renderButton({ starts })
  const button = screen.getByRole('button', { name: '생성' })
  await waitFor(() => expect(button).toBeEnabled())

  await userEvent.setup().click(button)

  await waitFor(() => expect(onStarted).toHaveBeenCalledWith('job-new'))
  expect(starts).toEqual([
    {
      postSlug: post.slug,
      observeModel: { providerId: 'openrouter', modelId: 'vision' },
      writeModelA: { providerId: 'openrouter', modelId: 'writer' },
      writeModelB: { providerId: 'openrouter', modelId: 'writer-b' },
    },
  ])
})

it('omits the observe model for a zero-photo post', async () => {
  const starts: FakeGenerationStart[] = []
  const draft = { ...post, images: [] }
  renderButton({ starts, draft })
  const button = screen.getByRole('button', { name: '생성' })
  await waitFor(() => expect(button).toBeEnabled())

  await userEvent.setup().click(button)

  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0]?.observeModel).toBeUndefined()
})
