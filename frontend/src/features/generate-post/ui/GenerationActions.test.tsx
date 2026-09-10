import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { Stage } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeGenerationStart } from '@/test/jobs'
import type { FakeWriteExperimentStart } from '@/test/experiments'
import { GenerationActions } from './GenerationActions'

afterEach(cleanup)

function setup(signedVideoUrl: boolean) {
  const starts: FakeGenerationStart[] = []
  const comparisons: FakeWriteExperimentStart[] = []
  const beforeStart = vi.fn(async () => {})
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
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { starts, comparisons, beforeStart, observe }
}

it('explains URL incompatibility and refuses both actions before saving', async () => {
  const user = userEvent.setup()
  const { starts, comparisons, beforeStart } = setup(false)
  expect(await screen.findByRole('status')).toHaveTextContent('영상 링크')
  for (const name of ['생성', 'A/B 비교']) {
    const button = screen.getByRole('button', { name })
    expect(button).toBeDisabled()
    await user.click(button)
  }
  expect(beforeStart).not.toHaveBeenCalled()
  expect(starts).toHaveLength(0)
  expect(comparisons).toHaveLength(0)
})

it.each(['생성', 'A/B 비교'])(
  'sends the observation model for a video-only %s request',
  async (name) => {
    const user = userEvent.setup()
    const { starts, comparisons, beforeStart, observe } = setup(true)
    const button = screen.getByRole('button', { name })
    await waitFor(() => expect(button).toBeEnabled())
    await user.click(button)
    const requests = name === '생성' ? starts : comparisons
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].observeModel).toEqual(observe)
    expect(beforeStart).toHaveBeenCalledTimes(1)
  },
)
