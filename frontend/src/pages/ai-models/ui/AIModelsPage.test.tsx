import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import { Stage } from '@/shared/api'
import type { FakeAnalyzeExperimentStart, FakeWriteExperimentStart } from '@/test/experiments'
import { renderAppAt } from '@/test/app'

const writeModels = [
  { providerId: 'openrouter', modelId: 'writer-a', label: 'Writer A' },
  { providerId: 'openrouter', modelId: 'writer-b', label: 'Writer B' },
]

const writePair = {
  stage: Stage.WRITE,
  candidateA: { providerId: 'openrouter', modelId: 'writer-a' },
  candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
}

it('starts a no-photo write comparison from the model tab with the persisted target length', async () => {
  const user = userEvent.setup()
  const starts: FakeWriteExperimentStart[] = []
  const { router } = renderAppAt('/ai-models', {
    user: { id: 'owner-1' },
    posts: {
      posts: [{ slug: 'post-1', title: '첫 글', targetLength: 1_600 }],
    },
    providers: { models: writeModels, comparisonPairs: [writePair] },
    experiments: { starts, experimentId: 'write-experiment-1' },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  expect(starts).toHaveLength(0)
  await user.selectOptions(screen.getByLabelText('비교할 글'), 'post-1')

  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)

  await waitFor(() =>
    expect(starts).toEqual([
      {
        postSlug: 'post-1',
        observeModel: undefined,
        modelA: { providerId: 'openrouter', modelId: 'writer-a' },
        modelB: { providerId: 'openrouter', modelId: 'writer-b' },
        targetLength: 1_600,
      },
    ]),
  )
  await waitFor(() =>
    expect(router.state.location.pathname).toBe('/ai-models/experiments/write-experiment-1'),
  )
})

it('requires and sends the explicit active observe model for a post with photos', async () => {
  const user = userEvent.setup()
  const starts: FakeWriteExperimentStart[] = []
  renderAppAt('/ai-models', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'photo-post',
          title: '사진 글',
          images: [{ id: 'image-1', filename: 'photo.jpg' }],
        },
      ],
    },
    providers: {
      models: [
        ...writeModels,
        { providerId: 'openrouter', modelId: 'vision', label: 'Vision', vision: true },
      ],
      selections: [{ stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'vision' }],
      comparisonPairs: [writePair],
    },
    experiments: { starts },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await user.selectOptions(screen.getByLabelText('비교할 글'), 'photo-post')
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)

  await waitFor(() =>
    expect(starts[0]?.observeModel).toEqual({
      providerId: 'openrouter',
      modelId: 'vision',
    }),
  )
})

it('keeps photo-backed writing blocked without an active observe model and reports start errors', async () => {
  const user = userEvent.setup()
  renderAppAt('/ai-models', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'photo-post',
          title: '사진 글',
          images: [{ id: 'image-1', filename: 'photo.jpg' }],
        },
        { slug: 'text-post', title: '텍스트 글' },
      ],
    },
    providers: { models: writeModels, comparisonPairs: [writePair] },
    experiments: { startError: 'provider unavailable' },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await user.selectOptions(screen.getByLabelText('비교할 글'), 'photo-post')
  expect(await screen.findByText('관찰 모델을 선택하세요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '비교 시작' })).toHaveAttribute('aria-disabled', 'true')

  await user.selectOptions(screen.getByLabelText('비교할 글'), 'text-post')
  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)
  expect(await screen.findByText('네트워크에 연결할 수 없어요.')).toBeInTheDocument()
})

it('blocks posts with active work or an unresolved write experiment', async () => {
  const user = userEvent.setup()
  const starts: FakeWriteExperimentStart[] = []
  renderAppAt('/ai-models', {
    user: { id: 'owner-1' },
    posts: {
      posts: [
        {
          slug: 'busy-post',
          title: '작업 중인 글',
          activeJob: { id: 'job-1', status: 'running' },
        },
        {
          slug: 'pending-post',
          title: '결과 대기 글',
          pendingExperimentId: 'experiment-pending',
        },
      ],
    },
    providers: { models: writeModels, comparisonPairs: [writePair] },
    experiments: { starts },
  })

  await user.click(await screen.findByRole('tab', { name: '글 작성' }))
  await user.selectOptions(screen.getByLabelText('비교할 글'), 'busy-post')
  expect(await screen.findByText('이미 생성 중이에요.')).toBeInTheDocument()

  await user.selectOptions(screen.getByLabelText('비교할 글'), 'pending-post')
  expect(await screen.findByText('먼저 대기 중인 A/B 결과를 확인해 주세요.')).toBeInTheDocument()
  expect(starts).toHaveLength(0)
})

const analyzePair = {
  stage: Stage.ANALYZE,
  candidateA: { providerId: 'openrouter', modelId: 'writer-a' },
  candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
}

const twoVoices = [
  { id: 'voice-default', name: '기본 말투', isDefault: true },
  { id: 'voice-review', name: '리뷰' },
]

// Plan 10 A13: an analyze comparison names one voice — initialized to the default.
it('starts an analyze comparison for the default voice unless another is chosen', async () => {
  const user = userEvent.setup()
  const analyzeStarts: FakeAnalyzeExperimentStart[] = []
  const { router } = renderAppAt('/ai-models', {
    user: { id: 'owner-1' },
    providers: { models: writeModels, comparisonPairs: [analyzePair] },
    voice: { voices: twoVoices },
    experiments: { analyzeStarts, experimentId: 'analyze-experiment-1' },
  })

  await user.click(await screen.findByRole('tab', { name: '문체 분석' }))
  const voice = await screen.findByLabelText('말투')
  await waitFor(() => expect(voice).toHaveValue('voice-default'))
  expect(screen.queryByLabelText('비교할 글')).not.toBeInTheDocument()

  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)

  await waitFor(() =>
    expect(analyzeStarts).toEqual([
      {
        voiceId: 'voice-default',
        modelA: { providerId: 'openrouter', modelId: 'writer-a' },
        modelB: { providerId: 'openrouter', modelId: 'writer-b' },
      },
    ]),
  )
  await waitFor(() =>
    expect(router.state.location.pathname).toBe('/ai-models/experiments/analyze-experiment-1'),
  )
})

it('sends the explicitly chosen voice with an analyze comparison', async () => {
  const user = userEvent.setup()
  const analyzeStarts: FakeAnalyzeExperimentStart[] = []
  renderAppAt('/ai-models', {
    user: { id: 'owner-1' },
    providers: { models: writeModels, comparisonPairs: [analyzePair] },
    voice: { voices: twoVoices },
    experiments: { analyzeStarts },
  })

  await user.click(await screen.findByRole('tab', { name: '문체 분석' }))
  const voice = await screen.findByLabelText('말투')
  await waitFor(() => expect(voice).toHaveValue('voice-default'))
  await user.selectOptions(voice, 'voice-review')

  const start = screen.getByRole('button', { name: '비교 시작' })
  await waitFor(() => expect(start).not.toHaveAttribute('aria-disabled'))
  await user.click(start)

  await waitFor(() => expect(analyzeStarts[0]?.voiceId).toBe('voice-review'))
})
