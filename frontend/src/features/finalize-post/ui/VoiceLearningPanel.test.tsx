import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import type { PostDraft } from '@/entities/post'
import { Stage } from '@/shared/api'
import { stubLearningHandoff } from '@/test/editor'
import { POST_CONTENT_FIXTURE } from '@/test/fixtures/postContent'
import type { FakeGenerationJobRow } from '@/test/jobs'
import { finalizedPostRow, type FakePostRow } from '@/test/posts'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { useVoiceLearning } from '../model/useVoiceLearning'
import { VoiceLearningPanel } from './VoiceLearningPanel'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const ANALYZER = {
  models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
  selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
}

/** A finalized post as the editor holds it, at `revision`. */
function draft(overrides: Partial<PostDraft> = {}): PostDraft {
  return {
    slug: '20260820-final',
    title: '가제',
    memo: '',
    status: 'finalized',
    voice: { id: 'voice-default', name: '기본 말투', deleted: false, sourceLanguage: 'ko' },
    template: { id: '', name: '' },
    images: [],
    videos: [],
    observations: [],
    content: POST_CONTENT_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    // The baseline was written under the post's own voice, so learning reads it as a correction.
    machineBaselineVoiceId: 'voice-default',
    finalizedRevision: 1n,
    canFinalize: true,
    targetLanguage: 'ko',
    contentLanguage: 'ko',
    ...overrides,
  } as unknown as PostDraft
}

function Panel({ post }: { post: PostDraft }) {
  const learning = useVoiceLearning('alice', post)
  return <VoiceLearningPanel learning={learning} onBackToRefine={() => {}} />
}

function renderPanel(post: PostDraft, row: FakePostRow, jobs: FakeGenerationJobRow[]) {
  const calls: string[] = []
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    calls,
    posts: { posts: [row] },
    jobs: { jobs },
    providers: ANALYZER,
  })
  render(<Panel post={post} />, { wrapper: withProviders(transport, createTestQueryClient()) })
  return { calls }
}

const FAILED: FakeGenerationJobRow = {
  id: 'learn-1',
  kind: 'voice_learn',
  status: 'failed',
  failureReason: 'MODEL_UNAVAILABLE',
}
const DONE: FakeGenerationJobRow = { id: 'learn-1', kind: 'voice_learn', status: 'done' }

it('keeps a failed learning handoff across reloads so only learning can be retried', async () => {
  const key = 'postpilot:voice-learning:alice:20260820-final'
  stubLearningHandoff({ [key]: JSON.stringify({ eventId: 'event-1', jobId: 'learn-1' }) })
  renderPanel(draft(), finalizedPostRow({ slug: '20260820-final' }), [FAILED])

  expect(await screen.findByText('AI 모델을 잠시 사용할 수 없어요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '다시 시도' })).toBeEnabled()
  expect(localStorage.getItem(key)).not.toBeNull()
})

it('removes a failed-learning retry when the current voice language is ineligible', async () => {
  const key = 'postpilot:voice-learning:alice:20260820-mismatch'
  stubLearningHandoff({
    [key]: JSON.stringify({ eventId: 'event-1', jobId: 'learn-1', contentRevision: '1' }),
  })
  const post = draft({ slug: '20260820-mismatch', contentLanguage: 'en' })
  const { calls } = renderPanel(
    post,
    finalizedPostRow({
      slug: '20260820-mismatch',
      contentLanguage: 'en',
      voice: { id: 'voice-default', name: '기본 말투', sourceLanguage: 'ko' },
    }),
    [FAILED],
  )

  expect(await screen.findByText('글과 말투의 언어가 달라 학습할 수 없어요.')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: '말투 학습' })).toBeDisabled()
  expect(calls).not.toContain('RetryVoiceLearning')
})

// The one thing 글 완성 can do, and it is done: the button stays put and says so. The completed
// run is read back from the handoff a reload preserved, so it cannot be started again.
it('keeps 말투 학습 disabled for a revision it has already learned from', async () => {
  const key = 'postpilot:voice-learning:alice:20260820-final'
  stubLearningHandoff({
    [key]: JSON.stringify({ eventId: 'event-1', jobId: 'learn-1', contentRevision: '1' }),
  })
  renderPanel(draft(), finalizedPostRow({ slug: '20260820-final' }), [DONE])

  expect(await screen.findByText('이 글에서 말투를 배웠어요.')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '말투 학습' })).toBeDisabled()
})

it('does not let a completed handoff from an older revision hide later learning', async () => {
  const key = 'postpilot:voice-learning:alice:20260820-final'
  stubLearningHandoff({
    [key]: JSON.stringify({ eventId: 'event-1', jobId: 'learn-1', contentRevision: '1' }),
  })
  renderPanel(
    draft({ contentRevision: 2n, machineBaselineRevision: 2n, finalizedRevision: 2n }),
    finalizedPostRow({
      slug: '20260820-final',
      contentRevision: 2n,
      machineBaselineRevision: 2n,
      finalizedRevision: 2n,
    }),
    [DONE],
  )

  const learn = await screen.findByRole('button', { name: '말투 학습' })
  await waitFor(() => expect(learn).toBeEnabled())
})
