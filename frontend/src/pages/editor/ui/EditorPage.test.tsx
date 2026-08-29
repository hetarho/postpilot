import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { discardUploadBatches } from '@/features/upload-photos'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import {
  OBSERVATION_FIXTURE,
  POST_CONTENT_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
import { FAKE_STORAGE_ORIGIN } from '@/test/posts'
import { clearCaret } from '../model/editor-handoff'

const USER = { id: 'alice' }

afterEach(() => {
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
  discardUploadBatches()
  vi.unstubAllGlobals()
})

/** jsdom has no image decoder, canvas encoder or object URLs; these stand in for the
 *  browser so the test can follow a file through the whole upload handshake. */
function stubBrowserImagePipeline() {
  vi.stubGlobal(
    'createImageBitmap',
    vi.fn(async () => ({ width: 4032, height: 3024, close: () => {} })),
  )
  vi.stubGlobal(
    'OffscreenCanvas',
    class {
      getContext() {
        return { fillRect() {}, drawImage() {}, fillStyle: '' }
      }
      convertToBlob = async () => new Blob(['jpeg'], { type: 'image/jpeg' })
    },
  )
  // jsdom's URL lacks these two; the class itself must stay (the router constructs URLs).
  URL.createObjectURL = () => 'blob:preview'
  URL.revokeObjectURL = () => {}
  const put = vi.fn(async () => new Response(null, { status: 200 }))
  vi.stubGlobal('fetch', put)
  return { put }
}

describe('opening a post', () => {
  // A2 (title/memo half of plan 02 AC11).
  it('restores the title and the memo', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ slug: '20260820-jeju', title: '제주 3일', memo: '첫날은 비' }] },
    })

    expect(await screen.findByLabelText('제목')).toHaveValue('제주 3일')
    expect(screen.getByLabelText('메모')).toHaveValue('첫날은 비')
    expect(screen.queryByRole('heading', { name: '내보내기' })).not.toBeInTheDocument()
  })

  it('switches export formats without making another client request', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'review',
            createdAt: '2026-08-20T12:00:00Z',
            content: POST_CONTENT_FIXTURE,
          },
        ],
      },
    })

    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    await waitFor(() => {
      expect(calls).toEqual(
        expect.arrayContaining(['GetPost', 'ListModels', 'GetSelections', 'GetVoiceProfile']),
      )
    })
    calls.length = 0

    await user.click(screen.getByRole('tab', { name: '티스토리' }))
    await user.click(screen.getByRole('tab', { name: '자체 사이트' }))
    await user.click(screen.getByRole('tab', { name: '마크다운' }))

    expect(calls).toEqual([])
  })

  it('resumes polling the active job exposed by the post', async () => {
    const calls: string[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            activeJob: {
              id: 'job-1',
              status: 'running',
              stage: 'observe',
              progressDone: 2,
              progressTotal: 4,
            },
          },
        ],
      },
      jobs: {
        jobs: [
          {
            id: 'job-1',
            status: 'running',
            stage: 'observe',
            progressDone: 2,
            progressTotal: 4,
          },
        ],
      },
    })

    expect(await screen.findByText('사진 2/4 관찰됨')).toBeInTheDocument()
    expect(calls).toContain('GetGeneration')
  })

  it('resumes an active revision beside the rendered content', async () => {
    const active = {
      id: 'revision-1',
      kind: 'revise',
      status: 'running',
      stage: 'write',
      progressDone: 0,
      progressTotal: 1,
    }
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            activeJob: active,
          },
        ],
      },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'writer' },
          { providerId: 'openrouter', modelId: 'writer-b' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'writer' },
            candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
          },
        ],
      },
      jobs: { jobs: [active] },
    })

    expect(await screen.findByRole('heading', { name: '비 온 뒤의 제주' })).toBeInTheDocument()
    expect(screen.getByText('작성 중')).toBeInTheDocument()
    expect(screen.getByLabelText('수정 요청')).toBeDisabled()
  })

  it('does not offer a no-op retry when a resumed revision fails', async () => {
    const resumed = {
      id: 'revision-1',
      kind: 'revise',
      status: 'running',
      stage: 'write',
      progressDone: 0,
      progressTotal: 1,
    }
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            activeJob: resumed,
          },
        ],
      },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'writer' },
          { providerId: 'openrouter', modelId: 'writer-b' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'writer' },
            candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
          },
        ],
      },
      jobs: {
        jobs: [{ ...resumed, status: 'failed', error: 'provider failed' }],
      },
    })

    expect(await screen.findByText('provider failed')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('수정 요청')).toBeEnabled()
  })

  it('routes a failed A/B job to its existing recovery screen', async () => {
    const user = userEvent.setup()
    const failed = {
      id: 'comparison-job',
      kind: 'model_experiment',
      status: 'failed',
      error: 'candidate failed',
    }
    const { router } = renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            activeJob: failed,
            pendingExperimentId: 'experiment-pending',
          },
        ],
      },
      jobs: { jobs: [failed] },
    })

    await user.click(await screen.findByRole('button', { name: '다시 시도' }))
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/ai-models/experiments/experiment-pending'),
    )
  })

  it('refreshes observations while running and renders the review draft after completion', async () => {
    const slug = '20260820-jeju'
    const image = { id: 'img-1', filename: 'IMG_1.jpg' }
    const active = {
      id: 'job-1',
      status: 'running',
      stage: 'observe',
      progressDone: 0,
      progressTotal: 1,
    }
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      posts: {
        posts: [{ slug, images: [image], activeJob: active }],
        getSequence: [
          { slug, images: [image], activeJob: active },
          { slug, images: [image], activeJob: active },
          {
            slug,
            images: [image],
            observations: [OBSERVATION_FIXTURE],
            activeJob: { ...active, progressDone: 1 },
          },
          {
            slug,
            status: 'review',
            images: [image],
            observations: [OBSERVATION_FIXTURE],
            content: POST_CONTENT_FIXTURE,
          },
        ],
      },
      jobs: {
        sequence: [
          { ...active, progressDone: 1 },
          { id: 'job-1', status: 'done', stage: 'write', progressDone: 1, progressTotal: 1 },
        ],
      },
    })

    expect(await screen.findByText('비가 그친 바닷가')).toBeInTheDocument()
    expect(screen.queryByText('관찰 대기')).not.toBeInTheDocument()
    expect(await screen.findByText('검토', {}, { timeout: 4_000 })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '비 온 뒤의 제주' })).toBeInTheDocument()
  })

  it('keeps the empty-profile warning non-blocking for zero-photo generation', async () => {
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      posts: { posts: [{ slug: '20260820-memo', memo: '사진 없는 메모' }] },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'writer' },
          { providerId: 'openrouter', modelId: 'writer-b' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'writer' },
            candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
          },
        ],
      },
    })

    expect(await screen.findByText(/문체 프로필이 비어 있어요/)).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '생성' })).toBeEnabled())
    expect(screen.getByText('사진이 없어 관찰 모델은 필요하지 않아요.')).toBeInTheDocument()
    expect(screen.getByLabelText('저장 상태와 글 작업').previousElementSibling).toHaveClass(
      'mt-auto',
      'h-6',
    )
  })

  it('flushes the newest memo before it starts generation', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: { posts: [{ slug: '20260820-memo', memo: '처음 메모' }] },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'writer' },
          { providerId: 'openrouter', modelId: 'writer-b' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'writer' },
            candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
          },
        ],
      },
    })

    const generate = await screen.findByRole('button', { name: '생성' })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.type(screen.getByLabelText('메모'), ' + 최신 내용')
    await user.click(generate)

    await waitFor(() => expect(calls).toContain('StartGeneration'))
    expect(calls.filter((call) => call === 'SavePostDraft' || call === 'StartGeneration')).toEqual([
      'SavePostDraft',
      'StartGeneration',
    ])
  })

  it('keeps ordinary generation usable when only the A/B pair is missing', async () => {
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      posts: { posts: [{ slug: '20260820-memo' }] },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'writer-old' },
          { providerId: 'openrouter', modelId: 'writer-new' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer-old' }],
      },
    })

    const generate = await screen.findByRole('button', { name: '생성' })
    const compare = screen.getByRole('button', { name: 'A/B 비교 생성' })
    await waitFor(() => expect(generate).toBeEnabled())
    expect(compare).toBeDisabled()
    expect(screen.getByText(/A\/B 비교: 작성 A\/B 모델 두 개를 선택하세요/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'AI 모델에서 두 후보 설정' })).toHaveAttribute(
      'href',
      '/ai-models',
    )
  })

  it('sends the active writer only for ordinary generation', async () => {
    const starts: Array<{
      postSlug: string
      writeModel?: { providerId: string; modelId: string }
      targetLength?: number
    }> = []
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: { posts: [{ slug: '20260820-memo' }] },
      jobs: { starts },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'active' },
          { providerId: 'openrouter', modelId: 'candidate-a' },
          { providerId: 'openrouter', modelId: 'candidate-b' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'active' }],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'candidate-a' },
            candidateB: { providerId: 'openrouter', modelId: 'candidate-b' },
          },
        ],
      },
    })
    const generate = await screen.findByRole('button', { name: '생성' })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.click(generate)
    await waitFor(() => expect(calls).toContain('StartGeneration'))
    expect(starts).toEqual([
      {
        postSlug: '20260820-memo',
        writeModel: { providerId: 'openrouter', modelId: 'active' },
        targetLength: undefined,
      },
    ])
    expect(calls).not.toContain('StartWriteExperiment')
  })

  it('starts an explicit A/B comparison with the configured pair and optional target', async () => {
    const starts: Array<{
      postSlug: string
      modelA?: { providerId: string; modelId: string }
      modelB?: { providerId: string; modelId: string }
      targetLength?: number
    }> = []
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: { posts: [{ slug: '20260820-memo' }] },
      experiments: { starts },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'active' },
          { providerId: 'openrouter', modelId: 'candidate-a' },
          { providerId: 'openrouter', modelId: 'candidate-b' },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'active' }],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'candidate-a' },
            candidateB: { providerId: 'openrouter', modelId: 'candidate-b' },
          },
        ],
      },
    })
    const options = await screen.findByRole('button', { name: '생성 옵션' })
    await user.click(options)
    await user.click(screen.getByRole('checkbox'))
    await user.type(screen.getByLabelText('목표 글자 수'), '750')
    await user.click(
      within(screen.getByRole('dialog', { name: '생성 옵션' })).getByRole('button', {
        name: '저장',
      }),
    )
    await waitFor(() => expect(calls).toContain('SavePostGenerationOptions'))
    const compare = screen.getByRole('button', { name: 'A/B 비교 생성' })
    await waitFor(() => expect(compare).toBeEnabled())
    await user.click(compare)
    await waitFor(() => expect(calls).toContain('StartWriteExperiment'))
    expect(starts).toEqual([
      {
        postSlug: '20260820-memo',
        observeModel: undefined,
        modelA: { providerId: 'openrouter', modelId: 'candidate-a' },
        modelB: { providerId: 'openrouter', modelId: 'candidate-b' },
        targetLength: 750,
      },
    ])
    expect(calls).not.toContain('StartGeneration')
  })

  it('restores and explicitly clears a stored target length without starting generation', async () => {
    const calls: string[] = []
    const generationOptionSaves: Array<number | undefined> = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: {
        posts: [{ slug: '20260820-memo', targetLength: 1200 }],
        generationOptionSaves,
      },
    })

    await user.click(await screen.findByRole('button', { name: '생성 옵션' }))
    const dialog = screen.getByRole('dialog', { name: '생성 옵션' })
    expect(within(dialog).getByRole('checkbox')).toBeChecked()
    expect(within(dialog).getByLabelText('목표 글자 수')).toHaveValue(1200)
    expect(calls).not.toContain('SavePostGenerationOptions')

    await user.click(within(dialog).getByRole('checkbox'))
    await user.click(within(dialog).getByRole('button', { name: '저장' }))
    await waitFor(() => expect(generationOptionSaves).toEqual([undefined]))
    expect(calls).not.toContain('StartGeneration')
    expect(calls).not.toContain('StartWriteExperiment')
  })

  it('finalizes without an analyze model or learning call', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
          },
        ],
      },
    })
    const finalize = await screen.findByRole('button', { name: '확정' })
    expect(finalize).toBeEnabled()
    expect(screen.getByRole('button', { name: '확정하고 말투 학습' })).toBeDisabled()
    await user.click(finalize)
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: '확정' }))
    await waitFor(() => expect(calls).toContain('FinalizePost'))
    expect(calls).not.toContain('LearnFromFinalizedPost')
    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeDisabled()
  })

  it('keeps the post finalized when explicit learning fails', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
          },
        ],
      },
      voice: { learningFails: true },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })
    const combined = await screen.findByRole('button', { name: '확정하고 말투 학습' })
    await waitFor(() => expect(combined).toBeEnabled())
    await user.click(combined)
    await user.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: '확정하고 학습' }),
    )
    await waitFor(() =>
      expect(calls).toEqual(expect.arrayContaining(['FinalizePost', 'LearnFromFinalizedPost'])),
    )
    expect(screen.getByText(/글은 확정됐지만 말투 학습은 시작하지 못했어요/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 학습' })).toBeEnabled()
  })

  it('returns a finalized post to review after the first changed content save', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
            finalizedRevision: 1n,
            finalizedAt: '2026-08-20T12:00:00Z',
          },
        ],
      },
    })

    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeInTheDocument()
    await user.type(screen.getByLabelText('본문 제목'), ' 수정')
    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })
    expect(await screen.findByRole('button', { name: '확정' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '말투 학습' })).not.toBeInTheDocument()
  })

  it('keeps a failed learning handoff across reloads so only learning can be retried', async () => {
    const key = 'postpilot:voice-learning:alice:20260820-final'
    const stored = new Map<string, string>()
    vi.stubGlobal('localStorage', {
      get length() {
        return stored.size
      },
      key: (index: number) => [...stored.keys()][index] ?? null,
      getItem: (name: string) => stored.get(name) ?? null,
      setItem: (name: string, value: string) => stored.set(name, value),
      removeItem: (name: string) => stored.delete(name),
    })
    localStorage.setItem(key, JSON.stringify({ eventId: 'event-1', jobId: 'learn-1' }))
    renderAppAt('/posts/20260820-final', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
            finalizedRevision: 1n,
            finalizedAt: '2026-08-20T12:00:00Z',
          },
        ],
      },
      jobs: {
        jobs: [{ id: 'learn-1', kind: 'voice_learn', status: 'failed', error: 'provider failed' }],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    expect(await screen.findByText('provider failed')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: '확정' })).not.toBeInTheDocument()
    expect(localStorage.getItem(key)).not.toBeNull()
    localStorage.removeItem(key)
  })

  it('does not let a completed handoff from an older revision hide later learning', async () => {
    const key = 'postpilot:voice-learning:alice:20260820-final'
    const stored = new Map<string, string>()
    vi.stubGlobal('localStorage', {
      get length() {
        return stored.size
      },
      key: (index: number) => [...stored.keys()][index] ?? null,
      getItem: (name: string) => stored.get(name) ?? null,
      setItem: (name: string, value: string) => stored.set(name, value),
      removeItem: (name: string) => stored.delete(name),
    })
    localStorage.setItem(
      key,
      JSON.stringify({ eventId: 'event-1', jobId: 'learn-1', contentRevision: '1' }),
    )
    renderAppAt('/posts/20260820-final', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-final',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 2n,
            machineBaselineRevision: 2n,
            canFinalize: true,
            finalizedRevision: 2n,
            finalizedAt: '2026-08-20T12:00:00Z',
          },
        ],
      },
      jobs: { jobs: [{ id: 'learn-1', kind: 'voice_learn', status: 'done' }] },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    const learn = await screen.findByRole('button', { name: '말투 학습' })
    await waitFor(() => expect(learn).toBeEnabled())
    localStorage.removeItem(key)
  })

  // Job 05 A6 (plan 02 AC11, photos half): the strip is rebuilt from the view URLs.
  it('restores its photos in the strip from their view URLs', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            images: [
              {
                id: 'img-1',
                filename: 'IMG_1.jpg',
                viewUrl: `${FAKE_STORAGE_ORIGIN}/posts/20260820-jeju/img-1.jpg?sig`,
              },
              { id: 'img-2', filename: 'IMG_2.jpg' },
            ],
          },
        ],
      },
    })

    expect(await screen.findByRole('img', { name: 'IMG_1.jpg' })).toHaveAttribute(
      'src',
      `${FAKE_STORAGE_ORIGIN}/posts/20260820-jeju/img-1.jpg?sig`,
    )
    expect(screen.getByRole('img', { name: 'IMG_2.jpg' })).toBeInTheDocument()
  })

  // Job 05 A5 (plan 02 AC6, browser half): delete calls DeleteImage and the photo is gone.
  it('deletes a photo from the strip through DeleteImage', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        calls,
        posts: [
          {
            slug: '20260820-jeju',
            images: [
              { id: 'img-1', filename: 'IMG_1.jpg' },
              { id: 'img-2', filename: 'IMG_2.jpg' },
            ],
          },
        ],
      },
    })

    // Deleting a photo is confirmed through the sheet: the × sits exactly where a thumb lands
    // when flicking the strip sideways, and the delete is not undoable.
    await user.click(await screen.findByRole('button', { name: 'IMG_1.jpg 삭제' }))
    await user.click(await screen.findByRole('button', { name: '삭제' }))

    await waitFor(() =>
      expect(screen.queryByRole('img', { name: 'IMG_1.jpg' })).not.toBeInTheDocument(),
    )
    expect(screen.getByRole('img', { name: 'IMG_2.jpg' })).toBeInTheDocument()
    expect(calls).toContain('DeleteImage')
  })

  it('keeps a photo whose delete failed and says so', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        deleteFails: true,
        posts: [{ slug: '20260820-jeju', images: [{ id: 'img-1', filename: 'IMG_1.jpg' }] }],
      },
    })

    await user.click(await screen.findByRole('button', { name: 'IMG_1.jpg 삭제' }))
    await user.click(await screen.findByRole('button', { name: '삭제' }))

    // The sheet stays open on failure and says so in place, so the retry is one tap away.
    expect(await screen.findByRole('alert')).toHaveTextContent('삭제하지 못했어요')
    await user.click(screen.getByRole('button', { name: '취소' }))
    expect(screen.getByRole('img', { name: 'IMG_1.jpg' })).toBeInTheDocument()
  })

  // A5. Someone else's slug is 403, not 404 (spec/policy/posts.md).
  it('reports a slug that belongs to someone else as theirs, not as missing', async () => {
    renderAppAt('/posts/20260101-hers', {
      user: USER,
      posts: { foreign: ['20260101-hers'] },
    })

    expect(await screen.findByRole('alert')).toHaveTextContent('다른 사람의 글이에요')
    expect(screen.getByRole('link', { name: '글 목록으로' })).toBeInTheDocument()
  })

  it('reports an unknown slug as missing', async () => {
    renderAppAt('/posts/20260101-ghost', { user: USER })

    expect(await screen.findByRole('alert')).toHaveTextContent('없는 글이에요')
  })

  // Only a failure that is not an answer is worth asking again.
  it('offers a retry only when the failure was not an answer', async () => {
    renderAppAt('/posts/20260101-ghost', { user: USER })

    await screen.findByRole('alert')
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()
  })
})

// Real timers on purpose: these walk the whole flow through the router, and
// @testing-library's async helpers look for jest's fake-timer API, so vitest's is
// invisible to them and every `waitFor` would spin on a clock nothing advances. The
// debounce window itself is covered by features/save-draft's own tests.
describe('a new draft', () => {
  /** The debounce plus room for the create round trip. */
  const AUTOSAVED = { timeout: 4_000 }

  // A3: the first autosave creates the post and the URL follows, with no reload.
  it('creates the post on the first autosave and moves the URL to the minted slug', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts/new', { user: USER })

    await user.type(await screen.findByLabelText('제목'), '제주 3일')

    await waitFor(
      () => expect(router.state.location.pathname).toBe('/posts/20260828-제주-3일'),
      AUTOSAVED,
    )
    // The text is what the editor came up with, not something refetched later.
    expect(screen.getByLabelText('제목')).toHaveValue('제주 3일')
    // …and the caret is still where the user left it, so the next keystroke lands.
    expect(screen.getByLabelText('제목')).toHaveFocus()
  })

  // Job 05 A2/A4 through the editor: the first photo of a new draft creates the post,
  // then CreateUpload → PUT to the storage host → ConfirmUpload, and the photo lands in
  // the strip of the editor the mint navigation mounted.
  it('creates the post on the first photo, uploads it straight to storage, and shows it', async () => {
    const { put } = stubBrowserImagePipeline()
    const calls: string[] = []
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts/new', { user: USER, posts: { calls } })

    // A JPEG, so the stubbed native decoder is the path taken; the HEIC worker path is
    // covered by shared/lib/image's own tests (jsdom has no Worker).
    await user.upload(
      await screen.findByLabelText('사진 추가'),
      new File(['jpeg'], 'IMG_1.JPG', { type: 'image/jpeg' }),
    )

    // Shown from the local copy until a GetPost brings a presigned URL.
    expect(await screen.findByRole('img', { name: 'IMG_1.jpg' })).toHaveAttribute(
      'src',
      expect.stringMatching(new RegExp(`^(blob:preview|${FAKE_STORAGE_ORIGIN})`)),
    )
    expect(router.state.location.pathname).toBe('/posts/20260828-untitled')
    const handshake = new Set(['SavePostDraft', 'CreateUpload', 'ConfirmUpload'])
    expect(calls.filter((call) => handshake.has(call))).toEqual([
      'SavePostDraft',
      'CreateUpload',
      'ConfirmUpload',
    ])
    // The bytes went to the storage host with the signed Content-Type, never to the API.
    expect(put).toHaveBeenCalledWith(
      expect.stringContaining(FAKE_STORAGE_ORIGIN),
      expect.objectContaining({ method: 'PUT', headers: { 'Content-Type': 'image/jpeg' } }),
    )
    // The confirmed photo is part of the post now: it can be deleted like any other.
    expect(screen.getByRole('button', { name: 'IMG_1.jpg 삭제' })).toBeInTheDocument()
  })

  it('says the post is being created while the first save is retried, then uploads', async () => {
    stubBrowserImagePipeline()
    const user = userEvent.setup()
    renderAppAt('/posts/new', { user: USER, posts: { failSaves: 1 } })

    await user.upload(await screen.findByLabelText('사진 추가'), new File(['x'], 'IMG_1.jpg'))

    expect(await screen.findByText('글을 만드는 중…')).toBeInTheDocument()
    // The retry lands after the backoff and the photo goes on to upload.
    await waitFor(
      () => expect(screen.getByRole('img', { name: 'IMG_1.jpg' })).toBeInTheDocument(),
      { timeout: 4_000 },
    )
  })

  // Job 05 A1: a pick with nothing to upload is reported and creates no post.
  it('lists a pick made only of skipped files without creating a post', async () => {
    const calls: string[] = []
    // The input's `accept` would hide an .exe in a real picker too; some pickers ignore
    // it, which is what the filter is for.
    const user = userEvent.setup({ applyAccept: false })
    const { router } = renderAppAt('/posts/new', { user: USER, posts: { calls } })

    await user.upload(await screen.findByLabelText('사진 추가'), new File(['x'], 'setup.exe'))

    expect(await screen.findByRole('heading', { name: '건너뜀' })).toBeInTheDocument()
    expect(screen.getByText('setup.exe')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/posts/new')
    expect(calls).not.toContain('SavePostDraft')
  })

  it('keeps typing in the same post rather than creating another', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts/new', { user: USER, posts: { calls } })

    await user.type(await screen.findByLabelText('제목'), '제주')
    await waitFor(
      () => expect(router.state.location.pathname).toBe('/posts/20260828-제주'),
      AUTOSAVED,
    )

    await user.type(screen.getByLabelText('메모'), '첫날은 비')
    await waitFor(
      () => expect(calls.filter((call) => call === 'SavePostDraft')).toHaveLength(2),
      AUTOSAVED,
    )
    expect(screen.getByLabelText('메모')).toHaveValue('첫날은 비')
  })
})
