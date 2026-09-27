import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
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

type User = ReturnType<typeof userEvent.setup>

/** A/B 비교 lives in the ▾ beside 바로 글 쓰기: open it, then choose the row. */
async function pressComparison(user: User) {
  const trigger = screen.getByRole('button', { name: '다른 방법으로 쓰기' })
  await waitFor(() => expect(trigger).toBeEnabled())
  await user.click(trigger)
  const menu = await screen.findByRole('menu', { name: '다른 방법으로 쓰기' })
  await user.click(within(menu).getByRole('menuitem', { name: 'A/B 비교' }))
}

async function press(user: User, name: '바로 글 쓰기' | '스토리라인 먼저' | 'A/B 비교') {
  if (name === 'A/B 비교') return pressComparison(user)
  const button = screen.getByRole('button', { name })
  await waitFor(() => expect(button).toBeEnabled())
  await user.click(button)
}

function setup(signedVideoUrl: boolean) {
  const starts: FakeGenerationStart[] = []
  const storylineStarts: FakeGenerationStart[] = []
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
    jobs: { starts, storylineStarts },
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
  return { starts, storylineStarts, comparisons, beforeStart, onOpenBrief, observe }
}

// A refusal for the setup is not said under the row: the press opens the brief for its own run,
// where the field that cannot serve the post is marked.
it('sends a press its setup refused to the brief, before saving anything', async () => {
  const user = userEvent.setup()
  const { starts, storylineStarts, comparisons, beforeStart, onOpenBrief } = setup(false)
  // 스토리라인 먼저 needs what 바로 글 쓰기 needs, so the brief marks that run's fields.
  for (const [name, mode] of [
    ['바로 글 쓰기', 'generation'],
    ['스토리라인 먼저', 'generation'],
    ['A/B 비교', 'comparison'],
  ] as const) {
    await press(user, name)
    expect(onOpenBrief).toHaveBeenLastCalledWith(mode)
  }
  expect(screen.queryByRole('status')).toBeNull()
  expect(screen.queryByText(/영상 링크/)).toBeNull()
  expect(beforeStart).not.toHaveBeenCalled()
  expect(starts).toHaveLength(0)
  expect(storylineStarts).toHaveLength(0)
  expect(comparisons).toHaveLength(0)
})

it.each(['바로 글 쓰기', '스토리라인 먼저', 'A/B 비교'] as const)(
  'sends the observation model for a video-only %s request',
  async (name) => {
    const user = userEvent.setup()
    const { starts, storylineStarts, comparisons, beforeStart, onOpenBrief, observe } = setup(true)
    await press(user, name)
    const requests =
      name === '바로 글 쓰기' ? starts : name === '스토리라인 먼저' ? storylineStarts : comparisons
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].observeModel).toEqual(observe)
    expect(beforeStart).toHaveBeenCalledTimes(1)
    expect(onOpenBrief).not.toHaveBeenCalled()
    // A comparison started here writes the post the editor is on, so its verdict applies.
    if (name === 'A/B 비교') expect(comparisons[0].origin).toBe(ExperimentOrigin.EDITOR)
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
  const storylineStarts: FakeGenerationStart[] = []
  const onStarted = vi.fn()
  const comparisons: FakeWriteExperimentStart[] = []
  const calls: string[] = []
  const beforeStart = vi.fn(async () => {})
  const onOpenBrief = vi.fn()
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    calls,
    providers,
    jobs: { starts, storylineStarts },
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
      onStarted={onStarted}
      beforeStart={beforeStart}
      onOpenBrief={onOpenBrief}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { starts, storylineStarts, comparisons, calls, beforeStart, onOpenBrief, onStarted }
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
  const generate = screen.getByRole('button', { name: '바로 글 쓰기' })
  await waitFor(() => expect(generate).toBeEnabled())
  expect(screen.getByRole('button', { name: '스토리라인 먼저' })).toBeEnabled()
  expect(screen.queryByText('작성 A/B 모델 두 개를 선택하세요.')).toBeNull()

  await user.click(screen.getByRole('button', { name: '다른 방법으로 쓰기' }))
  const compare = within(await screen.findByRole('menu')).getByRole('menuitem', {
    name: 'A/B 비교',
  })
  expect(compare).not.toHaveAttribute('aria-disabled')
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
  await press(user, '바로 글 쓰기')

  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0]).toMatchObject({ postSlug: 'post', writeModel: writer, targetLength: undefined })
  expect(calls).not.toContain('StartWriteExperiment')
  expect(calls).not.toContain('StartStoryline')
})

// GEN-68: 스토리라인 먼저 starts a storyline job with the active writer and the observer, and hands
// the job to the editor like any other start.
it('starts a storyline job from 스토리라인 먼저', async () => {
  const user = userEvent.setup()
  const { starts, storylineStarts, onStarted, beforeStart } = renderActions({
    images: POST_IMAGES_FIXTURE,
  })
  await press(user, '스토리라인 먼저')

  await waitFor(() => expect(storylineStarts).toHaveLength(1))
  expect(storylineStarts[0]).toMatchObject({
    postSlug: 'post',
    writeModel: writer,
    observeModel: observer,
  })
  expect(storylineStarts[0].reobserveFiles).toBeUndefined()
  expect(beforeStart).toHaveBeenCalledTimes(1)
  expect(starts).toHaveLength(0)
  await waitFor(() => expect(onStarted).toHaveBeenCalledWith('job-started'))
})

// GEN-8: a post that has been observed decides what to re-observe BEFORE the enqueue, and the
// picker confirmed untouched reuses everything — EMPTY, not absent, which would mean "observe all".
it('sends an EMPTY frozen set when the picker is confirmed untouched', async () => {
  const user = userEvent.setup()
  const { starts } = renderActions({
    images: POST_IMAGES_FIXTURE,
    observations: OBSERVATIONS_FIXTURE,
  })
  await press(user, '바로 글 쓰기')
  await user.click(await screen.findByRole('button', { name: '이대로 시작' }))

  await waitFor(() => expect(starts).toHaveLength(1))
  expect(starts[0].reobserveFiles).toEqual([])
})

// 스토리라인 먼저 shares the picker and its reuse contract.
it('routes 스토리라인 먼저 through the same picker', async () => {
  const user = userEvent.setup()
  const { storylineStarts } = renderActions({
    images: POST_IMAGES_FIXTURE,
    observations: OBSERVATIONS_FIXTURE,
  })
  await press(user, '스토리라인 먼저')
  await user.click(await screen.findByRole('button', { name: '이대로 시작' }))

  await waitFor(() => expect(storylineStarts).toHaveLength(1))
  expect(storylineStarts[0].reobserveFiles).toEqual([])
})

// A8: the A/B comparison shares the picker and the same reuse contract.
it('routes the A/B comparison through the same picker', async () => {
  const user = userEvent.setup()
  const { comparisons } = renderActions({
    images: POST_IMAGES_FIXTURE,
    observations: OBSERVATIONS_FIXTURE,
  })
  await pressComparison(user)
  await user.click(await screen.findByRole('button', { name: '이대로 시작' }))

  await waitFor(() => expect(comparisons).toHaveLength(1))
  expect(comparisons[0].reobserveFiles).toEqual([])
})

// A pending A/B result holds every action, so the picker never opens and nothing is saved or
// enqueued.
it('keeps every action disabled while an A/B result is pending', async () => {
  const user = userEvent.setup()
  const { starts, calls, beforeStart } = renderActions({
    images: POST_IMAGES_FIXTURE,
    observations: OBSERVATIONS_FIXTURE,
    pendingExperimentId: 'experiment-1',
  })
  const generate = screen.getByRole('button', { name: '바로 글 쓰기' })
  await waitFor(() => expect(calls).toContain('GetSelections'))
  expect(generate).toBeDisabled()
  expect(screen.getByRole('button', { name: '스토리라인 먼저' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '다른 방법으로 쓰기' })).toBeDisabled()
  await user.click(generate)
  expect(screen.queryByRole('dialog', { name: '다시 관찰할 사진 선택' })).not.toBeInTheDocument()
  expect(beforeStart).not.toHaveBeenCalled()
  expect(calls).not.toContain('StartGeneration')
  expect(starts).toHaveLength(0)
})

// GEN-8: nothing to reuse means no picker.
it('starts directly when the post has photos but no stored observation', async () => {
  const user = userEvent.setup()
  const { starts } = renderActions({ images: POST_IMAGES_FIXTURE })
  await press(user, '바로 글 쓰기')

  await waitFor(() => expect(starts).toHaveLength(1))
  expect(screen.queryByRole('dialog', { name: '다시 관찰할 사진 선택' })).not.toBeInTheDocument()
  expect(starts[0].reobserveFiles).toBeUndefined()
})
