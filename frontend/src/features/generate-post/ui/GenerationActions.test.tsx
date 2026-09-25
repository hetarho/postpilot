import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import type { PostDraft } from '@/entities/post'
import { ExperimentOrigin, Stage } from '@/shared/api'
import { OBSERVATIONS_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import type { FakeProvidersOptions } from '@/test/providers'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeGenerationStart } from '@/test/jobs'
import type { FakeWriteExperimentStart } from '@/test/experiments'
import { GenerationActions } from './GenerationActions'

afterEach(cleanup)

function setup(signedVideoUrl: boolean) {
  const starts: FakeGenerationStart[] = []
  const comparisons: FakeWriteExperimentStart[] = []
  const beforeStart = vi.fn(async () => {})
  const onOpenBrief = vi.fn()
  const observe = { providerId: 'p', modelId: 'watcher' }
  const write = { providerId: 'p', modelId: 'writer' }
  const writeB = { providerId: 'p', modelId: 'writer-b' }
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    providers: {
      models: [
        { ...observe, vision: true, videoInput: true, signedVideoUrl, inlineStaticVideo: true },
        write,
        writeB,
      ],
      selections: [
        { ...observe, stage: Stage.OBSERVE },
        { ...write, stage: Stage.WRITE },
      ],
      comparisonPairs: [{ stage: Stage.WRITE, candidateA: write, candidateB: writeB }],
    },
    jobs: { starts },
    experiments: { starts: comparisons },
  })
  render(
    <GenerationActions
      post={{
        slug: 'post',
        status: 'draft',
        images: [],
        observations: [],
        pendingExperimentId: '',
        voice: { id: 'voice', name: 'Voice', deleted: false, sourceLanguage: 'ko' },
        videos: [
          {
            id: 'video',
            filename: 'scene.mp4',
            width: 1280,
            height: 720,
            bytes: 3,
            durationMs: 1000,
            contentType: 'video/mp4',
            viewUrl: 'blob:local',
          },
        ],
      }}
      onStarted={vi.fn()}
      beforeStart={beforeStart}
      onOpenBrief={onOpenBrief}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { starts, comparisons, beforeStart, onOpenBrief, observe }
}

// A refusal for the setup is not said under the row: the press opens the brief for its own run,
// where the field that cannot serve the post is marked.
it('sends a press its setup refused to the brief, before saving anything', async () => {
  const user = userEvent.setup()
  const { starts, comparisons, beforeStart, onOpenBrief } = setup(false)
  for (const [name, mode] of [
    ['생성', 'generation'],
    ['A/B 비교', 'comparison'],
  ] as const) {
    const button = screen.getByRole('button', { name })
    await waitFor(() => expect(button).toBeEnabled())
    await user.click(button)
    expect(onOpenBrief).toHaveBeenLastCalledWith(mode)
  }
  expect(screen.queryByRole('status')).toBeNull()
  expect(screen.queryByText(/영상 링크/)).toBeNull()
  expect(beforeStart).not.toHaveBeenCalled()
  expect(starts).toHaveLength(0)
  expect(comparisons).toHaveLength(0)
})

it.each(['생성', 'A/B 비교'])(
  'sends the observation model for a video-only %s request',
  async (name) => {
    const user = userEvent.setup()
    const { starts, comparisons, beforeStart, onOpenBrief, observe } = setup(true)
    const button = screen.getByRole('button', { name })
    await waitFor(() => expect(button).toBeEnabled())
    await user.click(button)
    const requests = name === '생성' ? starts : comparisons
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].observeModel).toEqual(observe)
    expect(beforeStart).toHaveBeenCalledTimes(1)
    expect(onOpenBrief).not.toHaveBeenCalled()
    // A comparison started here writes the post the editor is on, so its verdict applies.
    if (name !== '생성') expect(comparisons[0].origin).toBe(ExperimentOrigin.EDITOR)
  },
)

type ActionsPost = Parameters<typeof GenerationActions>[0]['post']

const observer = { providerId: 'openrouter', modelId: 'observer' }
const writer = { providerId: 'openrouter', modelId: 'writer' }
const candidateA = { providerId: 'openrouter', modelId: 'candidate-a' }
const candidateB = { providerId: 'openrouter', modelId: 'candidate-b' }
/** An observer and a writer chosen, and a pair of two other writers. */
const PICKED: FakeProvidersOptions = {
  models: [{ ...observer, vision: true }, writer, candidateA, candidateB],
  selections: [
    { stage: Stage.OBSERVE, ...observer },
    { stage: Stage.WRITE, ...writer },
  ],
  comparisonPairs: [{ stage: Stage.WRITE, candidateA, candidateB }],
}

/** The actions over a post of the case's own, with every start recorded. */
function renderActions(post: Partial<ActionsPost> = {}, providers: FakeProvidersOptions = PICKED) {
  const starts: FakeGenerationStart[] = []
  const comparisons: FakeWriteExperimentStart[] = []
  const calls: string[] = []
  const beforeStart = vi.fn(async () => {})
  const onOpenBrief = vi.fn()
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    calls,
    providers,
    jobs: { starts },
    experiments: { starts: comparisons },
  })
  render(
    <GenerationActions
      post={{
        slug: 'post',
        status: 'draft' as PostDraft['status'],
        images: [],
        videos: [],
        observations: [],
        pendingExperimentId: '',
        voice: { id: 'voice', name: 'Voice', deleted: false, sourceLanguage: 'ko' },
        ...post,
      }}
      onStarted={vi.fn()}
      beforeStart={beforeStart}
      onOpenBrief={onOpenBrief}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { starts, comparisons, calls, beforeStart, onOpenBrief }
}

// The press is the way to the fix (owner decision 2026-09-25): an A/B 비교 with no pair stays live,
// says nothing under the row, and hands the brief its own run to mark.
it('keeps ordinary generation usable when only the A/B pair is missing', async () => {
  const user = userEvent.setup()
  const { starts, comparisons, onOpenBrief } = renderActions(
    {},
    {
      models: [writer, { providerId: 'openrouter', modelId: 'writer-new' }],
      selections: [{ stage: Stage.WRITE, ...writer }],
    },
  )
  const generate = screen.getByRole('button', { name: '생성' })
  const compare = screen.getByRole('button', { name: 'A/B 비교' })
  await waitFor(() => expect(generate).toBeEnabled())
  expect(compare).toBeEnabled()
  expect(screen.queryByText('작성 A/B 모델 두 개를 선택하세요.')).toBeNull()

  await user.click(compare)
  expect(onOpenBrief).toHaveBeenCalledWith('comparison')
  expect(starts).toHaveLength(0)
  expect(comparisons).toHaveLength(0)
})

it('sends the active writer only for ordinary generation', async () => {
  const user = userEvent.setup()
  const { starts, calls } = renderActions(
    {},
    { ...PICKED, selections: [{ stage: Stage.WRITE, ...writer }] },
  )
  const generate = screen.getByRole('button', { name: '생성' })
  await waitFor(() => expect(generate).toBeEnabled())
  await user.click(generate)

  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0]).toMatchObject({ postSlug: 'post', writeModel: writer, targetLength: undefined })
  expect(calls).not.toContain('StartWriteExperiment')
})

// Change 21: a post that has been observed decides what to re-observe BEFORE the enqueue, and the
// picker confirmed untouched reuses everything — EMPTY, not absent, which would mean "observe all".
it('sends an EMPTY frozen set when the picker is confirmed untouched', async () => {
  const user = userEvent.setup()
  const { starts } = renderActions({
    images: POST_IMAGES_FIXTURE,
    observations: OBSERVATIONS_FIXTURE,
  })
  const generate = screen.getByRole('button', { name: '생성' })
  await waitFor(() => expect(generate).toBeEnabled())
  await user.click(generate)
  await user.click(await screen.findByRole('button', { name: '이대로 시작' }))

  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0].reobserveFiles).toEqual([])
})

// A8: the A/B comparison shares the picker and the same reuse contract.
it('routes the A/B comparison through the same picker', async () => {
  const user = userEvent.setup()
  const { comparisons } = renderActions({
    images: POST_IMAGES_FIXTURE,
    observations: OBSERVATIONS_FIXTURE,
  })
  const compare = screen.getByRole('button', { name: 'A/B 비교' })
  await waitFor(() => expect(compare).toBeEnabled())
  await user.click(compare)
  await user.click(await screen.findByRole('button', { name: '이대로 시작' }))

  await waitFor(() => expect(comparisons).toHaveLength(1))
  expect(comparisons[0].reobserveFiles).toEqual([])
})

// A pending A/B result holds both actions, so the picker never opens and nothing is saved or
// enqueued.
it('keeps 생성 disabled while an A/B result is pending', async () => {
  const user = userEvent.setup()
  const { starts, calls, beforeStart } = renderActions({
    images: POST_IMAGES_FIXTURE,
    observations: OBSERVATIONS_FIXTURE,
    pendingExperimentId: 'experiment-1',
  })
  const generate = screen.getByRole('button', { name: '생성' })
  await waitFor(() => expect(calls).toContain('GetSelections'))
  expect(generate).toBeDisabled()
  await user.click(generate)
  expect(screen.queryByRole('dialog', { name: '다시 관찰할 사진 선택' })).not.toBeInTheDocument()
  expect(beforeStart).not.toHaveBeenCalled()
  expect(calls).not.toContain('StartGeneration')
  expect(starts).toHaveLength(0)
})

// A1/A10: nothing to reuse means no picker, exactly as before change 21.
it('starts directly when the post has photos but no stored observation', async () => {
  const user = userEvent.setup()
  const { starts } = renderActions({ images: POST_IMAGES_FIXTURE })
  const generate = screen.getByRole('button', { name: '생성' })
  await waitFor(() => expect(generate).toBeEnabled())
  await user.click(generate)

  await waitFor(() => expect(starts).toHaveLength(1))
  expect(screen.queryByRole('dialog', { name: '다시 관찰할 사진 선택' })).not.toBeInTheDocument()
  expect(starts[0].reobserveFiles).toBeUndefined()
})
