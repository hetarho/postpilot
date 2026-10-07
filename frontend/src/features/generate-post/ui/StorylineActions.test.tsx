import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import type { GenerationJob } from '@/entities/generation-job'
import { Stage } from '@/shared/api'
import type { FakeGenerationStart } from '@/test/jobs'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { StorylineActionButtons, StorylineActionsProvider } from './StorylineActions'

afterEach(cleanup)

const writer = { providerId: 'openrouter', modelId: 'writer' }

type ActionsPost = Parameters<typeof StorylineActionsProvider>[0]['post']

function renderStorylineActions(post: Partial<ActionsPost> = {}, activeJob?: GenerationJob) {
  const starts: FakeGenerationStart[] = []
  const storylineStarts: FakeGenerationStart[] = []
  const beforeStart = vi.fn(async () => {})
  const flushContent = vi.fn(async () => {})
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    providers: {
      models: [writer],
      selections: [{ stage: Stage.WRITE, ...writer }],
    },
    jobs: { starts, storylineStarts },
  })
  render(
    <StorylineActionsProvider
      post={{
        slug: 'post',
        status: 'draft',
        images: [],
        videos: [],
        observations: [],
        pendingExperimentId: 'frozen-test',
        voice: undefined,
        storyline: {
          paragraphs: [{ text: 'Keep the frozen material independent.', files: [] }],
          editedByHand: false,
          takenOutFiles: [],
          addedFiles: [],
        },
        content: undefined,
        contentRevision: 0n,
        machineBaselineRevision: 0n,
        ...post,
      }}
      activeJob={activeJob}
      onStarted={vi.fn()}
      beforeStart={beforeStart}
      flushContent={flushContent}
      onOpenBrief={vi.fn()}
    >
      <StorylineActionButtons />
    </StorylineActionsProvider>,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { starts, storylineStarts, beforeStart, flushContent }
}

it.each(['다시 만들기', '이 스토리로 글 쓰기'] as const)(
  'permits ordinary %s while an independent frozen test waits',
  async (name) => {
    const user = userEvent.setup()
    const { starts, storylineStarts, beforeStart, flushContent } = renderStorylineActions()
    const button = screen.getByRole('button', { name })
    await waitFor(() => expect(button).toBeEnabled())
    await user.click(button)
    await waitFor(() => expect(name === '다시 만들기' ? storylineStarts : starts).toHaveLength(1))
    expect(beforeStart).toHaveBeenCalledTimes(1)
    expect(flushContent).toHaveBeenCalledTimes(name === '다시 만들기' ? 0 : 1)
  },
)

it('keeps ordinary storyline actions guarded by an active writing job', async () => {
  const activeJob: GenerationJob = {
    id: 'ordinary',
    kind: 'generate',
    status: 'running',
    stage: 'write',
    progressDone: 0,
    progressTotal: 1,
    failure: undefined,
    postSlug: 'post',
    observeModel: undefined,
    writeModel: writer,
    createdAt: '',
    updatedAt: '',
    targetLanguage: undefined,
  }
  const { beforeStart } = renderStorylineActions({}, activeJob)
  expect(screen.getByRole('button', { name: '다시 만들기' })).toBeDisabled()
  expect(screen.getByRole('button', { name: '이 스토리로 글 쓰기' })).toBeDisabled()
  expect(beforeStart).not.toHaveBeenCalled()
})
