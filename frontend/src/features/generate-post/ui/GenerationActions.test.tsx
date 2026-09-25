import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { ExperimentOrigin, Stage } from '@/shared/api'
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
