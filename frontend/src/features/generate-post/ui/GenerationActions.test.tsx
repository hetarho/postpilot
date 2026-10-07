import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import type { PostDraft } from '@/entities/post'
import { Stage } from '@/shared/api'
import { OBSERVATIONS_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import type { FakeProvidersOptions } from '@/test/providers'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeGenerationStart } from '@/test/jobs'
import type { FakeWriteExperimentStart } from '@/test/experiments'
import { GenerationActions } from './GenerationActions'

afterEach(cleanup)

type User = ReturnType<typeof userEvent.setup>

async function press(user: User, name: '바로 글 쓰기' | '스토리라인 먼저') {
  const button = screen.getByRole('button', { name })
  await waitFor(() => expect(button).toBeEnabled())
  await user.click(button)
}

function setup(signedVideoUrl: boolean, checkRequiredAnswers = vi.fn(() => true)) {
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
        voice: { id: 'voice', name: 'Voice', deleted: false, made: true },
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
      checkRequiredAnswers={checkRequiredAnswers}
      onOpenBrief={onOpenBrief}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return {
    starts,
    storylineStarts,
    comparisons,
    beforeStart,
    checkRequiredAnswers,
    onOpenBrief,
    observe,
  }
}

it('checks required template answers before ordinary writing or planning', async () => {
  const user = userEvent.setup()
  const checkRequiredAnswers = vi.fn(() => false)
  const { starts, storylineStarts, comparisons, beforeStart } = setup(true, checkRequiredAnswers)
  for (const name of ['바로 글 쓰기', '스토리라인 먼저'] as const) {
    await press(user, name)
  }
  expect(checkRequiredAnswers).toHaveBeenCalledTimes(2)
  expect(beforeStart).not.toHaveBeenCalled()
  expect(starts).toHaveLength(0)
  expect(storylineStarts).toHaveLength(0)
  expect(comparisons).toHaveLength(0)
})

// A refusal for the setup is not said under the row: the press opens the brief for its own run,
// where the field that cannot serve the post is marked.
it('sends a press its setup refused to the brief, before saving anything', async () => {
  const user = userEvent.setup()
  const { starts, storylineStarts, comparisons, beforeStart, onOpenBrief } = setup(false)
  // 스토리라인 먼저 needs what 바로 글 쓰기 needs, so the brief marks that run's fields.
  for (const [name, mode] of [
    ['바로 글 쓰기', 'generation'],
    ['스토리라인 먼저', 'generation'],
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

it.each(['바로 글 쓰기', '스토리라인 먼저'] as const)(
  'sends the observation model for a video-only %s request',
  async (name) => {
    const user = userEvent.setup()
    const { starts, storylineStarts, beforeStart, onOpenBrief, observe } = setup(true)
    await press(user, name)
    const requests = name === '바로 글 쓰기' ? starts : storylineStarts
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].observeModel).toEqual(observe)
    expect(beforeStart).toHaveBeenCalledTimes(1)
    expect(onOpenBrief).not.toHaveBeenCalled()
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
function renderActions(
  post: Partial<ActionsPost> = {},
  providers: FakeProvidersOptions = PICKED,
  props: Partial<Omit<Parameters<typeof GenerationActions>[0], 'post'>> = {},
) {
  const starts: FakeGenerationStart[] = []
  const storylineStarts: FakeGenerationStart[] = []
  const onStarted = vi.fn()
  const comparisons: FakeWriteExperimentStart[] = []
  const calls: string[] = []
  const beforeStart = vi.fn(props.beforeStart ?? (async () => {}))
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
      {...props}
      post={{
        slug: 'post',
        status: 'draft' as PostDraft['status'],
        images: [],
        videos: [],
        observations: [],
        pendingExperimentId: '',
        voice: { id: 'voice', name: 'Voice', deleted: false, made: true },
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

it('keeps both ordinary actions usable without legacy candidate pairs or a start menu', async () => {
  const user = userEvent.setup()
  const { starts, comparisons, onOpenBrief } = renderActions(
    { images: POST_IMAGES_FIXTURE },
    {
      models: [writer, { ...observer, vision: true }],
      selections: [
        { stage: Stage.WRITE, ...writer },
        { stage: Stage.OBSERVE, ...observer },
      ],
    },
  )
  await press(user, '바로 글 쓰기')
  expect(screen.getByRole('button', { name: '스토리라인 먼저' })).toBeEnabled()
  expect(screen.queryByRole('button', { name: '다른 방법으로 쓰기' })).not.toBeInTheDocument()
  expect(screen.queryByRole('menuitem', { name: 'A/B 비교' })).not.toBeInTheDocument()
  await waitFor(() => expect(starts).toHaveLength(1))
  expect(onOpenBrief).not.toHaveBeenCalled()
  expect(comparisons).toHaveLength(0)
})

it('sends the active writer only for ordinary generation', async () => {
  const user = userEvent.setup()
  const { starts, calls } = renderActions(
    {},
    { ...PICKED, selections: [{ stage: Stage.WRITE, ...writer }] },
  )
  await press(user, '바로 글 쓰기')
  const dialog = await screen.findByRole('dialog', { name: '사진 없이 만들까요?' })
  await user.click(within(dialog).getByRole('button', { name: '사진 없이 만들기' }))

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

it.each(['바로 글 쓰기', '스토리라인 먼저'] as const)(
  'allows %s while a retained test result awaits review',
  async (name) => {
    const user = userEvent.setup()
    const { starts, storylineStarts, calls, beforeStart } = renderActions({
      images: POST_IMAGES_FIXTURE,
      observations: OBSERVATIONS_FIXTURE,
      pendingExperimentId: 'experiment-1',
    })
    const result = screen.getByRole('link', { name: 'A/B 결과 확인' })
    expect(result).toHaveAttribute('href', '/posts/experiments/experiment-1')
    await press(user, name)
    expect(beforeStart).not.toHaveBeenCalled()
    await user.click(await screen.findByRole('button', { name: '이대로 시작' }))
    const requests = name === '바로 글 쓰기' ? starts : storylineStarts
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(beforeStart).toHaveBeenCalledTimes(1)
    expect(calls).not.toContain('StartWriteExperiment')
  },
)

// GEN-8: nothing to reuse means no picker.
it('starts directly when the post has photos but no stored observation', async () => {
  const user = userEvent.setup()
  const { starts } = renderActions({ images: POST_IMAGES_FIXTURE })
  await press(user, '바로 글 쓰기')

  await waitFor(() => expect(starts).toHaveLength(1))
  expect(screen.queryByRole('dialog', { name: '다시 관찰할 사진 선택' })).not.toBeInTheDocument()
  expect(starts[0].reobserveFiles).toBeUndefined()
})

// POST-108: a run with nothing attached asks once; cancelling saves and starts nothing, and
// confirming starts the run the press asked for.
it.each(['바로 글 쓰기', '스토리라인 먼저'] as const)(
  'asks before a %s run with no photo or video attached',
  async (name) => {
    const user = userEvent.setup()
    const { starts, storylineStarts, beforeStart } = renderActions()
    const requests = name === '바로 글 쓰기' ? starts : storylineStarts

    await press(user, name)
    let dialog = await screen.findByRole('dialog', { name: '사진 없이 만들까요?' })
    expect(dialog).toHaveTextContent('첨부된 사진이 없어요.')
    await user.click(within(dialog).getByRole('button', { name: '취소' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: '사진 없이 만들까요?' })).toBeNull(),
    )
    expect(beforeStart).not.toHaveBeenCalled()
    expect(requests).toHaveLength(0)

    await press(user, name)
    dialog = await screen.findByRole('dialog', { name: '사진 없이 만들까요?' })
    await user.click(within(dialog).getByRole('button', { name: '사진 없이 만들기' }))
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(beforeStart).toHaveBeenCalledTimes(1)
  },
)

it.each(['바로 글 쓰기', '스토리라인 먼저'] as const)(
  'awaits the newest draft flush before starting %s',
  async (name) => {
    const user = userEvent.setup()
    let finish!: () => void
    const saved = new Promise<void>((resolve) => {
      finish = resolve
    })
    const { starts, storylineStarts, beforeStart } = renderActions(
      { images: POST_IMAGES_FIXTURE },
      PICKED,
      { beforeStart: () => saved },
    )
    await press(user, name)
    expect(beforeStart).toHaveBeenCalledTimes(1)
    expect(starts).toHaveLength(0)
    expect(storylineStarts).toHaveLength(0)
    expect(screen.getByRole('button', { name })).toBeDisabled()
    await act(async () => {
      finish()
      await saved
    })
    const requests = name === '바로 글 쓰기' ? starts : storylineStarts
    await waitFor(() => expect(requests).toHaveLength(1))
  },
)

it('preserves the draft and reports a failed material flush without starting AI', async () => {
  const user = userEvent.setup()
  const { starts, storylineStarts, comparisons } = renderActions(
    { images: POST_IMAGES_FIXTURE },
    PICKED,
    {
      beforeStart: async () => {
        throw new Error('private cause')
      },
    },
  )
  await press(user, '바로 글 쓰기')
  const alert = await screen.findByRole('alert')
  expect(alert).toHaveTextContent('요청을 마치지 못했어요.')
  expect(alert).not.toHaveTextContent('private cause')
  expect(starts).toHaveLength(0)
  expect(storylineStarts).toHaveLength(0)
  expect(comparisons).toHaveLength(0)
})

it.each([
  { name: 'published post', post: { status: 'published' as const } },
  {
    name: 'deleted voice',
    post: { voice: { id: 'gone', name: 'Gone', deleted: true, made: true } },
  },
  {
    name: 'unmade voice',
    post: { voice: { id: 'making', name: 'Making', deleted: false, made: false } },
  },
])('keeps ordinary actions guarded for a $name alongside a retained result', async ({ post }) => {
  const { calls, beforeStart } = renderActions({ ...post, pendingExperimentId: 'retained' })
  await waitFor(() => expect(calls).toContain('GetSelections'))
  expect(screen.getByRole('button', { name: '바로 글 쓰기' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '스토리라인 먼저' })).toBeDisabled()
  expect(beforeStart).not.toHaveBeenCalled()
})
