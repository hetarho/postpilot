import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { discardUploadBatches } from '@/features/upload-photos'
import { create } from '@bufbuild/protobuf'
import {
  PublishJobSchema,
  PublishStatus,
  PublishVisibility,
  PublishingAgentSchema,
  Stage,
} from '@/shared/api'
import { renderAppAt } from '@/test/app'
import {
  OBSERVATION_FIXTURE,
  POST_CONTENT_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
import { FAKE_STORAGE_ORIGIN, type FakeDraftSave } from '@/test/posts'
import { clearCaret } from '../model/editor-handoff'

const USER = { id: 'alice' }

/** The editor is one mounted component showing three step panels, so a test reaches a panel the
 *  post's status did not open by pressing its step. */
async function openStep(user: ReturnType<typeof userEvent.setup>, label: string) {
  await user.click(await screen.findByRole('tab', { name: label }))
}

afterEach(() => {
  cleanup()
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
  discardUploadBatches()
  initializeI18n('ko')
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

    await openStep(user, '글 완성')
    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    // 글 완성 loads the post and the analyze selection the finalize control needs. The voice
    // profile is NOT among them any more: the empty-profile warning belongs to 글 생성, so a post
    // already past generating no longer pays for that read.
    await waitFor(() => {
      expect(calls).toEqual(expect.arrayContaining(['GetPost', 'ListModels', 'GetSelections']))
    })
    expect(calls).not.toContain('GetVoiceProfile')
    calls.length = 0

    await user.click(screen.getByRole('tab', { name: '티스토리' }))
    await user.click(screen.getByRole('tab', { name: '자체 사이트' }))
    await user.click(screen.getByRole('tab', { name: '마크다운' }))

    expect(calls).toEqual([])
  })

  it('renders publishing after export without starting on mount', async () => {
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

    await openStep(user, '글 완성')
    const exportHeading = await screen.findByRole('heading', { name: '내보내기' })
    const publishHeading = await screen.findByRole('heading', { name: '발행하기' })
    expect(
      exportHeading.compareDocumentPosition(publishHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    expect(calls).not.toContain('StartPublish')
  })

  it('starts only after the explicit final-publish confirmation', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const agent = create(PublishingAgentSchema, {
      id: 'agent-1',
      label: '침실 Mac',
      platformAccountId: 'my-blog',
      platformAccountLabel: '내 네이버 블로그',
      browserLabel: 'Google Chrome',
      categories: [{ id: 'daily', name: '일상' }],
      defaultCategoryId: 'daily',
      defaultVisibility: PublishVisibility.PUBLIC,
      lastSeenAt: new Date().toISOString(),
      ready: true,
    })
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'finalized',
            createdAt: '2026-08-20T12:00:00Z',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 3n,
            finalizedRevision: 3n,
          },
        ],
      },
      publishing: { agents: [agent] },
    })

    const publishButton = await screen.findByRole('button', { name: '네이버에 발행' })
    expect(calls).not.toContain('StartPublish')
    await user.click(publishButton)
    expect(calls).not.toContain('StartPublish')
    expect(screen.getByRole('dialog', { name: '네이버에 최종 발행할까요?' })).toHaveTextContent(
      '추가 확인을 요청하지 않습니다',
    )
    await user.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: '네이버에 발행' }),
    )
    await waitFor(() => expect(calls).toContain('StartPublish'))
  })

  it('flushes a pending edit and refuses to publish the formerly finalized revision', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    let releaseSave!: () => void
    const contentSaveGate = new Promise<void>((resolve) => {
      releaseSave = resolve
    })
    const agent = create(PublishingAgentSchema, {
      id: 'agent-1',
      label: '침실 Mac',
      platformAccountId: 'my-blog',
      platformAccountLabel: '내 네이버 블로그',
      browserLabel: 'Google Chrome',
      categories: [{ id: 'daily', name: '일상' }],
      defaultCategoryId: 'daily',
      defaultVisibility: PublishVisibility.PUBLIC,
      ready: true,
    })
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: {
        contentSaveGate,
        posts: [
          {
            slug: '20260820-jeju',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 3n,
            finalizedRevision: 3n,
            machineBaselineRevision: 3n,
          },
        ],
      },
      publishing: { agents: [agent] },
    })

    await openStep(user, '글 다듬기')
    await user.click(await screen.findByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '발행 직전에 고친 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })
    await openStep(user, '글 완성')
    await user.click(await screen.findByRole('button', { name: '네이버에 발행' }))

    expect(calls).not.toContain('StartPublish')
    expect(
      screen.queryByRole('dialog', { name: '네이버에 최종 발행할까요?' }),
    ).not.toBeInTheDocument()
    releaseSave()
    await waitFor(() =>
      expect(
        screen.getByText('현재 내용을 먼저 확정해야 정확히 이 버전을 발행할 수 있어요.'),
      ).toBeInTheDocument(),
    )
  })

  it('keeps the frozen category and visibility for a safe attention retry', async () => {
    const calls: string[] = []
    const startRequests: Array<{
      expectedContentRevision: bigint
      agentId: string
      categoryId: string
      visibility: number
    }> = []
    const user = userEvent.setup()
    const agent = create(PublishingAgentSchema, {
      id: 'agent-1',
      label: '침실 Mac',
      platformAccountId: 'my-blog',
      platformAccountLabel: '내 네이버 블로그',
      browserLabel: 'Google Chrome',
      categories: [
        { id: 'daily', name: '일상' },
        { id: 'travel', name: '여행' },
      ],
      defaultCategoryId: 'daily',
      defaultVisibility: PublishVisibility.PUBLIC,
      ready: true,
    })
    const job = create(PublishJobSchema, {
      id: 'publish-job-1',
      postSlug: '20260820-jeju',
      agentId: 'agent-1',
      status: PublishStatus.NEEDS_ATTENTION,
      contentRevision: 3n,
      categoryId: 'travel',
      visibility: PublishVisibility.PRIVATE,
    })
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'review',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 9n,
            finalizedRevision: 3n,
          },
        ],
      },
      publishing: { calls, agents: [agent], jobs: [job], startRequests },
    })

    await openStep(user, '글 완성')
    const retry = await screen.findByRole('button', { name: '안전하게 다시 시도' })
    expect(retry).toBeEnabled()
    expect(screen.getByLabelText('카테고리')).toHaveValue('travel')
    expect(screen.getByLabelText('카테고리')).toBeDisabled()
    expect(screen.getByLabelText('공개 설정')).toHaveValue(String(PublishVisibility.PRIVATE))
    expect(screen.getByLabelText('공개 설정')).toBeDisabled()

    await user.click(retry)
    await user.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: '네이버에 발행' }),
    )
    await waitFor(() => expect(calls).toContain('StartPublish'))
    expect(startRequests).toEqual([
      {
        expectedContentRevision: 3n,
        agentId: 'agent-1',
        categoryId: 'travel',
        visibility: PublishVisibility.PRIVATE,
      },
    ])
  })

  it('keeps pre-commit cancellation available when the paired agent is unavailable', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const job = create(PublishJobSchema, {
      id: 'publish-job-queued',
      postSlug: '20260820-jeju',
      agentId: 'revoked-agent',
      status: PublishStatus.QUEUED,
      contentRevision: 3n,
      categoryId: 'travel',
      visibility: PublishVisibility.PRIVATE,
    })
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 3n,
            finalizedRevision: 3n,
          },
        ],
      },
      publishing: { agents: [], jobs: [job] },
    })

    await user.click(await screen.findByRole('button', { name: '발행 취소' }))
    await waitFor(() => expect(calls).toContain('CancelPublish'))
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
        jobs: [{ ...resumed, status: 'failed', failureReason: 'MODEL_UNAVAILABLE' }],
      },
    })

    expect(await screen.findByText('AI 모델을 잠시 사용할 수 없어요.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('수정 요청')).toBeEnabled()
  })

  it('routes a failed A/B job to its existing recovery screen', async () => {
    const user = userEvent.setup()
    const failed = {
      id: 'comparison-job',
      kind: 'model_experiment',
      status: 'failed',
      failureReason: 'MODEL_OUTPUT_INVALID' as const,
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

    // The failure is reported on every step; its retry is mounted on the step that owns the job.
    expect(await screen.findByText('AI 결과 형식을 읽을 수 없어요.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()

    await openStep(user, '글 생성')
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
    // 확정 ends 글 다듬기, where the post opens.
    const finalize = await screen.findByRole('button', { name: '확정' })
    expect(finalize).toBeEnabled()
    expect(screen.getByRole('button', { name: '확정하고 말투 학습' })).toBeDisabled()
    await user.click(finalize)
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: '확정' }))
    await waitFor(() => expect(calls).toContain('FinalizePost'))
    expect(calls).not.toContain('LearnFromFinalizedPost')
    // Confirming carries the user to 글 완성, whose own action is learning — and this account has
    // no analyze model, so it is offered as disabled rather than hidden.
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: '글 완성' })).toHaveAttribute('aria-selected', 'true'),
    )
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
    // The failure is reported on 글 완성 — where the learning run lands — and the finalize it
    // followed still stands, so only learning is retried.
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: '글 완성' })).toHaveAttribute('aria-selected', 'true'),
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

    // A finalized post opens on 글 완성.
    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeInTheDocument()

    await openStep(user, '글 다듬기')
    await user.click(await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }))
    await user.type(screen.getByLabelText('본문 제목'), ' 수정')
    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })

    // Back in review, so 확정 is offered again where it belongs, and 글 완성 can no longer learn
    // from a revision the post has moved past.
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: '글 다듬기' })).toHaveAttribute(
        'aria-selected',
        'true',
      ),
    )
    expect(await screen.findByRole('button', { name: '확정' })).toBeInTheDocument()

    await openStep(user, '글 완성')
    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: '확정' })).not.toBeInTheDocument()
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
        jobs: [
          {
            id: 'learn-1',
            kind: 'voice_learn',
            status: 'failed',
            failureReason: 'MODEL_UNAVAILABLE',
          },
        ],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    expect(await screen.findByText('AI 모델을 잠시 사용할 수 없어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: '확정' })).not.toBeInTheDocument()
    expect(localStorage.getItem(key)).not.toBeNull()
    localStorage.removeItem(key)
  })

  it('removes a failed-learning retry when the current voice language is ineligible', async () => {
    const key = 'postpilot:voice-learning:alice:20260820-mismatch'
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
    const calls: string[] = []
    renderAppAt('/posts/20260820-mismatch', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            slug: '20260820-mismatch',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            finalizedRevision: 1n,
            contentLanguage: 'en',
            voice: {
              id: 'voice-default',
              name: '기본 말투',
              sourceLanguage: 'ko',
            },
          },
        ],
      },
      jobs: {
        jobs: [
          {
            id: 'learn-1',
            kind: 'voice_learn',
            status: 'failed',
            failureReason: 'MODEL_UNAVAILABLE',
          },
        ],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    expect(await screen.findByText('글과 말투의 언어가 달라 학습할 수 없어요.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 학습' })).toBeDisabled()
    expect(calls).not.toContain('RetryVoiceLearning')
    localStorage.removeItem(key)
  })

  // The one thing 글 완성 can do, and it is done: the button stays put and says so.
  it('keeps 말투 학습 disabled for a revision it has already learned from', async () => {
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
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
            finalizedRevision: 1n,
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

    // The completed run is read back from the handoff a reload preserved, so the outcome is on
    // screen and the button cannot start the same run again.
    expect(await screen.findByText('이 글에서 말투를 배웠어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 학습' })).toBeDisabled()
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
    const failure = await screen.findByRole('alert')
    expect(failure).toHaveTextContent('삭제하지 못했어요')
    expect(failure).toHaveTextContent('네트워크에 연결할 수 없어요.')
    expect(failure).not.toHaveTextContent('private backend prose')
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

describe('the editor lifecycle steps', () => {
  const reviewPost = {
    slug: '20260820-jeju',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
  }

  // Change 05 A1.
  it.each([
    ['draft', '글 생성'],
    ['review', '글 다듬기'],
    ['finalized', '글 완성'],
  ])('opens a %s post on %s', async (status, label) => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, status, canFinalize: true }] },
    })

    const tab = await screen.findByRole('tab', { name: label })
    await waitFor(() => expect(tab).toHaveAttribute('aria-selected', 'true'))
  })

  // Change 05 A2 / A3: each step renders its own panel and none of the others'.
  it('scopes the generation controls, the block surface, and finalize to their own steps', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, canFinalize: true }] },
    })

    // ② is where a review post opens: the draft, revision and the 확정 that ends the step, and
    // no generation controls.
    expect(await screen.findByRole('heading', { name: '글 다듬기' })).toBeInTheDocument()
    expect(screen.queryByLabelText('작성 모델')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '생성' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '내보내기' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '확정' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '확정하고 말투 학습' })).toBeInTheDocument()

    await openStep(user, '글 생성')
    expect(await screen.findByRole('heading', { name: '글 생성' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'AI 모델에서 두 후보 설정' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '글 다듬기' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '확정' })).not.toBeInTheDocument()

    await openStep(user, '글 완성')
    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 학습' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '확정' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '글 생성' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '생성' })).not.toBeInTheDocument()
  })

  // Change 05 A4: a step with no work yet says so and offers the way to the step that produces it.
  it('opens an empty step without touching the post', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: { posts: [{ slug: '20260820-jeju', status: 'draft', title: '제주 3일' }] },
    })

    await screen.findByRole('heading', { name: '글 생성' })
    calls.length = 0

    await openStep(user, '글 다듬기')
    expect(await screen.findByText(/아직 다듬을 글이 없어요/)).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: '글 다듬기' })).toBeEnabled()

    await user.click(screen.getByRole('button', { name: '글 생성으로 가기' }))
    expect(await screen.findByRole('heading', { name: '글 생성' })).toBeInTheDocument()

    // Opening a step is not an action: no status change, no job, no provider call.
    expect(calls).not.toContain('SavePostDraft')
    expect(calls).not.toContain('GeneratePost')
    expect(calls).not.toContain('SavePostContent')
  })

  // Change 05 A6: steps are panels, so a save started before a step change still completes.
  it('completes a title save started before the step changed', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: { posts: [{ ...reviewPost, title: '제주 3일' }] },
    })

    const title = await screen.findByLabelText('제목')
    await user.type(title, ' 여행기')
    await openStep(user, '글 완성')

    await waitFor(() => expect(calls).toContain('SavePostDraft'), { timeout: 4_000 })
    // The field is outside the panels, so it kept both its value and its pending save.
    expect(screen.getByLabelText('제목')).toHaveValue('제주 3일 여행기')
  })

  // Change 05 A11, as amended: the bar carries at most one committing action, and it is not
  // there at all on a step that has nothing to commit and nothing to report.
  it('docks one committing action and nothing on a quiet step', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, canFinalize: true }] },
    })

    // 글 다듬기 commits continuously through content autosave, and 확정 closes the step from
    // the end of the panel rather than from the bar — so an idle dock would be a card with
    // nothing in it.
    await screen.findByRole('heading', { name: '글 다듬기' })
    expect(screen.getByRole('button', { name: '확정' })).toBeInTheDocument()
    expect(screen.queryByLabelText('저장 상태와 글 작업')).not.toBeInTheDocument()

    await openStep(user, '글 생성')
    const dock = await screen.findByLabelText('저장 상태와 글 작업')
    expect(within(dock).getByRole('button', { name: '생성' })).toBeInTheDocument()
    expect(within(dock).queryByRole('button', { name: '확정' })).not.toBeInTheDocument()

    await openStep(user, '글 완성')
    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeInTheDocument()
    expect(screen.queryByLabelText('저장 상태와 글 작업')).not.toBeInTheDocument()
  })

  // The memo is what 글 생성 works from, so it lives there rather than above every step.
  it('shows the memo on 글 생성 only, without losing what was typed', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, memo: '비 오는 제주' }] },
    })

    await screen.findByRole('heading', { name: '글 다듬기' })
    expect(screen.queryByLabelText('메모')).not.toBeInTheDocument()

    await openStep(user, '글 생성')
    const memo = await screen.findByLabelText('메모')
    expect(memo).toHaveValue('비 오는 제주')
    await user.type(memo, ' 산책')

    await openStep(user, '글 다듬기')
    await openStep(user, '글 생성')
    expect(await screen.findByLabelText('메모')).toHaveValue('비 오는 제주 산책')
  })

  // The step bar is the first thing on the screen, above the post's title.
  it('puts the step bar above the title', async () => {
    renderAppAt('/posts/20260820-jeju', { user: USER, posts: { posts: [reviewPost] } })

    const tab = await screen.findByRole('tab', { name: '글 생성' })
    const title = screen.getByLabelText('제목')
    expect(tab.compareDocumentPosition(title) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})

describe('the draft read-first', () => {
  const reviewPost = {
    slug: '20260820-jeju',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
  }

  // Change 05 A7 / A10.
  it('renders the draft as prose with no form control until a block is opened', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', { user: USER, posts: { posts: [reviewPost] } })

    const draft = await screen.findByRole('article', { name: '생성된 글' })
    expect(within(draft).queryByRole('textbox')).not.toBeInTheDocument()
    expect(within(draft).getByText(POST_CONTENT_FIXTURE.blocks[0].content)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    expect(screen.getByLabelText('1번째 블록 내용')).toHaveValue(
      POST_CONTENT_FIXTURE.blocks[0].content,
    )
  })

  // Change 05 A8: cancel restores the value the block had when its editor opened.
  it('restores a cancelled block and keeps a saved one as prose', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', { user: USER, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '고쳐 쓴 문단')
    await user.click(screen.getByRole('button', { name: '취소' }))

    expect(screen.queryByLabelText('1번째 블록 내용')).not.toBeInTheDocument()
    expect(screen.getByText(POST_CONTENT_FIXTURE.blocks[0].content)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const reopened = screen.getByLabelText('1번째 블록 내용')
    await user.clear(reopened)
    await user.type(reopened, '확정한 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))

    expect(screen.queryByLabelText('1번째 블록 내용')).not.toBeInTheDocument()
    expect(screen.getByText('확정한 문단')).toBeInTheDocument()
  })

  // Change 05 A9: the editing UI keeps every capability it had.
  it('keeps add, delete and move available from a block editor', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', { user: USER, calls, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '2번째 블록 수정' }))
    expect(screen.getByRole('button', { name: '2번째 블록 위로' })).toBeEnabled()
    expect(screen.getByLabelText('2번째 블록 종류')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '삭제' }))
    expect(screen.queryByLabelText('2번째 블록 종류')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '문단 추가' }))
    expect(await screen.findByText('새 문단')).toBeInTheDocument()
    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })
  })
})

describe('the step split and the content save', () => {
  const reviewPost = {
    slug: '20260820-final',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
  }

  // 확정 sits at the end of the step the block editor lives on, so it flushes that editor's
  // pending save through its live ref — and the queue behind it — before it names a revision.
  it('saves an edited block before finalizing it', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', { user: USER, calls, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '확정 직전에 고친 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await user.click(await screen.findByRole('button', { name: '확정' }))
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: '확정' }))

    await waitFor(() => expect(calls).toContain('FinalizePost'))
    // The content save is ahead of the finalize, so the finalized revision includes the edit.
    expect(calls.indexOf('SavePostContent')).toBeGreaterThan(-1)
    expect(calls.indexOf('SavePostContent')).toBeLessThan(calls.indexOf('FinalizePost'))
  })

  // 취소 restores the block it opened on, so moving must close the editor rather than leave a
  // snapshot pointed at whichever block shifted into that slot.
  it('closes a block editor when the block moves', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', { user: USER, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '2번째 블록 수정' }))
    await user.click(screen.getByRole('button', { name: '2번째 블록 위로' }))

    expect(screen.queryByRole('button', { name: '취소' })).not.toBeInTheDocument()
    expect(screen.getByText(POST_CONTENT_FIXTURE.blocks[0].content)).toBeInTheDocument()
    expect(screen.getByText(POST_CONTENT_FIXTURE.blocks[1].content)).toBeInTheDocument()
  })

  // The header editor owns the title, summary and tags — cancelling it must not revert a block.
  it('keeps a block edit made while the header editor was open', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', { user: USER, posts: { posts: [reviewPost] } })

    await user.click(await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }))
    await user.type(screen.getByLabelText('본문 제목'), ' 수정')

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const block = screen.getByLabelText('1번째 블록 내용')
    await user.clear(block)
    await user.type(block, '유지되어야 하는 문단')
    await user.click(within(block.closest('article')!).getByRole('button', { name: '저장' }))

    // 취소 on the header restores its three fields only.
    await user.click(screen.getAllByRole('button', { name: '취소' })[0])

    expect(screen.getByText('유지되어야 하는 문단')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: POST_CONTENT_FIXTURE.title })).toBeInTheDocument()
  })
})

describe('the post voice', () => {
  const AUTOSAVED = { timeout: 4_000 }
  /** The picker lists the directory, which answers after the first paint; choose only once it has. */
  async function pickVoice(user: ReturnType<typeof userEvent.setup>, voiceId: string) {
    const picker = await screen.findByLabelText('말투')
    await waitFor(() => expect(picker).toBeEnabled())
    await screen.findByRole('option', { name: voiceId === 'voice-review' ? '리뷰' : '기본 말투' })
    await user.selectOptions(picker, voiceId)
    return picker
  }
  const TWO_VOICES = [
    { id: 'voice-default', name: '기본 말투', isDefault: true },
    { id: 'voice-review', name: '리뷰' },
  ]
  const POST_VOICES = [
    { id: 'voice-default', name: '기본 말투' },
    { id: 'voice-review', name: '리뷰' },
  ]
  const reviewPost = {
    slug: '20260820-jeju',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
  }

  // Plan 10 A4: the picker opens on the default and the create names it.
  it('starts a new draft in the default voice and sends its id with the first save', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    const { router } = renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })

    expect(await screen.findByLabelText('말투')).toHaveValue('voice-default')
    await user.type(screen.getByLabelText('제목'), '제주')

    await waitFor(
      () => expect(router.state.location.pathname).toBe('/posts/20260828-제주'),
      AUTOSAVED,
    )
    expect(draftSaves[0]).toEqual({
      slug: '',
      voiceId: 'voice-default',
      purposeId: undefined,
      targetLanguage: 'ko',
    })
    // The editor the mint mounted shows the same voice, and later saves leave it alone.
    expect(screen.getByLabelText('말투')).toHaveValue('voice-default')
    await user.type(screen.getByLabelText('메모'), '첫날')
    await waitFor(() => expect(draftSaves).toHaveLength(2), AUTOSAVED)
    expect(draftSaves[1]).toEqual({
      slug: '20260828-제주',
      voiceId: undefined,
      purposeId: undefined,
      targetLanguage: undefined,
    })
  })

  it('lets a new draft pick another voice before anything is typed', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    const { router } = renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves, voices: POST_VOICES },
      voice: { voices: TWO_VOICES },
    })

    await pickVoice(user, 'voice-review')
    // Choosing a voice is not typing: no post exists until there is something to save.
    expect(draftSaves).toHaveLength(0)
    await user.type(screen.getByLabelText('제목'), '리뷰 글')

    await waitFor(
      () =>
        expect(draftSaves[0]).toEqual({
          slug: '',
          voiceId: 'voice-review',
          purposeId: undefined,
          targetLanguage: 'ko',
        }),
      AUTOSAVED,
    )
    await waitFor(() => expect(router.state.location.pathname).toBe('/posts/20260828-리뷰-글'))
    expect(await screen.findByLabelText('말투')).toHaveValue('voice-review')
  })

  // Plan 10 A8: reassignment is confirmed, preserves the content, and clears learn eligibility.
  it('reassigns an existing post after confirmation and clears its learn eligibility', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [reviewPost], draftSaves, voices: POST_VOICES },
      voice: { voices: TWO_VOICES },
    })

    const picker = await screen.findByLabelText('말투')
    await waitFor(() => expect(picker).toHaveValue('voice-default'))
    await pickVoice(user, 'voice-review')
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('지금까지 배운 내용은 이전 말투에 남고')
    // The choice is not applied until it is confirmed.
    expect(draftSaves).toHaveLength(0)
    await user.click(within(dialog).getByRole('button', { name: '말투 변경' }))

    await waitFor(() =>
      expect(draftSaves).toEqual([{ slug: '20260820-jeju', voiceId: 'voice-review' }]),
    )
    await waitFor(() => expect(screen.getByLabelText('말투')).toHaveValue('voice-review'))
    // The canonical content survived; learning needs a new machine result first.
    expect(await screen.findByRole('button', { name: '확정' })).toBeEnabled()
    expect(screen.getByRole('button', { name: '확정하고 말투 학습' })).toBeDisabled()
    await openStep(user, '글 완성')
    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
  })

  it('keeps learning disabled when a finalized post is reassigned without a new baseline', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            ...reviewPost,
            status: 'finalized',
            finalizedRevision: reviewPost.contentRevision,
            finalizedAt: '2026-08-20T12:00:00Z',
          },
        ],
        voices: POST_VOICES,
      },
      voice: { voices: TWO_VOICES },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    await pickVoice(user, 'voice-review')
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '말투 변경' }),
    )
    await openStep(user, '글 완성')

    expect(
      await screen.findByText('새 말투로 다시 생성하거나 AI로 수정한 뒤에 학습할 수 있어요.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 학습' })).toBeDisabled()
  })

  it('cancels a reassignment from the sheet without saving anything', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [reviewPost], draftSaves, voices: POST_VOICES },
      voice: { voices: TWO_VOICES },
    })

    await pickVoice(user, 'voice-review')
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '취소' }),
    )

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.getByLabelText('말투')).toHaveValue('voice-default')
    await new Promise((resolve) => setTimeout(resolve, 1_200))
    expect(draftSaves).toHaveLength(0)
  })

  it('blocks reassignment while a job is active and says why', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [{ slug: '20260820-jeju', activeJob: { id: 'job-1', status: 'running' } }],
        voices: POST_VOICES,
      },
      voice: { voices: TWO_VOICES },
      jobs: { jobs: [{ id: 'job-1', status: 'running' }] },
    })

    expect(await screen.findByLabelText('말투')).toBeDisabled()
    expect(screen.getByText('AI 작업이 끝나면 말투를 바꿀 수 있어요.')).toBeInTheDocument()
  })

  it('reports a refused reassignment under the field and keeps the old voice', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      // The post service does not know 'voice-review': the directory is stale, and the server
      // answers NotFound rather than guessing.
      posts: { posts: [reviewPost] },
      voice: { voices: TWO_VOICES },
    })

    await pickVoice(user, 'voice-review')
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '말투 변경' }),
    )

    // `findAllByRole`: the dock's own save notice can be an alert at the same moment.
    const alerts = await screen.findAllByRole('alert')
    expect(alerts.map((alert) => alert.textContent).join('\n')).toContain(
      '고른 말투를 찾을 수 없어요',
    )
    expect(screen.getByLabelText('말투')).toHaveValue('voice-default')
    expect(screen.getByLabelText('말투')).toBeEnabled()
  })

  // Plan 10 A5/A7: the tombstone, the disabled AI controls with their reason, and both ways out.
  it('renders a deleted voice as a tombstone with disabled AI actions and a way out', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: {
        posts: [
          {
            ...reviewPost,
            status: 'draft',
            voice: { id: 'voice-old', name: '옛 말투', deleted: true },
          },
        ],
        voices: [...POST_VOICES, { id: 'voice-old', name: '옛 말투', deleted: true }],
      },
      voice: {
        voices: [...TWO_VOICES, { id: 'voice-old', name: '옛 말투', deleted: true }],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    // Named twice on purpose: in the picker (as the disabled current option) and in the warning.
    expect(await screen.findAllByText('삭제된 말투 · 옛 말투')).toHaveLength(2)
    expect(screen.getByLabelText('말투')).toHaveValue('voice-old')
    // The reason settles once the model catalog has answered; until then it reports the wait.
    expect(
      await screen.findByText('생성: 삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '생성' })).toBeDisabled()
    // The manual side of the post is untouched: its content and export are still there.
    await openStep(user, '글 완성')
    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    expect(
      screen.getByText('삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.'),
    ).toBeInTheDocument()
    await openStep(user, '글 다듬기')
    expect(
      await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '수정' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: '문장 의견' })).not.toBeInTheDocument()

    // Restore is offered in place and asks the server, nothing else ([I5]).
    await openStep(user, '글 생성')
    await user.click(screen.getByRole('button', { name: '복원' }))
    await waitFor(() => expect(calls).toContain('RestoreVoice'))
    expect(calls).not.toContain('StartGeneration')
  })

  it('recovers a deleted-voice post by reassigning it to an active voice', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            memo: '메모',
            voice: { id: 'voice-old', name: '옛 말투', deleted: true },
          },
        ],
        draftSaves,
        voices: [...POST_VOICES, { id: 'voice-old', name: '옛 말투', deleted: true }],
      },
      voice: { voices: [...TWO_VOICES, { id: 'voice-old', name: '옛 말투', deleted: true }] },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    await screen.findAllByText('삭제된 말투 · 옛 말투')
    await pickVoice(user, 'voice-review')
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '말투 변경' }),
    )

    await waitFor(() =>
      expect(draftSaves).toEqual([{ slug: '20260820-jeju', voiceId: 'voice-review' }]),
    )
    await waitFor(() => expect(screen.queryAllByText('삭제된 말투 · 옛 말투')).toHaveLength(0))
    await waitFor(() => expect(screen.getByRole('button', { name: '생성' })).toBeEnabled())
  })
})

describe('the post language', () => {
  const AUTOSAVED = { timeout: 4_000 }

  it('snapshots the current UI locale for a new post and sends it on the first request', async () => {
    initializeI18n('en')
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })

    expect(await screen.findByRole('combobox', { name: 'Post language' })).toHaveValue('en')
    await user.type(screen.getByLabelText('Title'), 'English draft')

    await waitFor(() => expect(draftSaves[0]?.targetLanguage).toBe('en'), AUTOSAVED)
  })

  it('sends an explicit pre-create language instead of the UI-locale default', async () => {
    initializeI18n('en')
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })

    await user.selectOptions(await screen.findByRole('combobox', { name: 'Post language' }), 'ko')
    await user.type(screen.getByLabelText('Title'), 'Korean target')

    await waitFor(() => expect(draftSaves[0]?.targetLanguage).toBe('ko'), AUTOSAVED)
  })

  it('restores an existing post language independently of the current UI locale', async () => {
    initializeI18n('en')
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/korean-post', {
      user: USER,
      posts: {
        posts: [{ slug: 'korean-post', title: '한국어 글', targetLanguage: 'ko' }],
        draftSaves,
      },
    })

    expect(await screen.findByRole('combobox', { name: 'Post language' })).toHaveValue('ko')
    expect(draftSaves).toHaveLength(0)
  })

  it('exports the frozen content provenance rather than a different current target', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/english-content', {
      user: USER,
      posts: {
        posts: [
          {
            slug: 'english-content',
            status: 'finalized',
            targetLanguage: 'ko',
            contentLanguage: 'en',
            content: POST_CONTENT_FIXTURE,
          },
        ],
      },
    })

    await openStep(user, '글 완성')
    await user.click(screen.getByRole('tab', { name: '자체 사이트' }))
    expect(screen.getByLabelText<HTMLTextAreaElement>('내보내기 결과').value).toContain(
      '<html lang="en">',
    )
    await user.click(screen.getByRole('tab', { name: '마크다운' }))
    expect(screen.getByLabelText<HTMLTextAreaElement>('내보내기 결과').value).toContain(
      '\nlanguage: en\n',
    )
  })

  it('fails closed instead of guessing when finalized content provenance is missing', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/malformed-content', {
      user: USER,
      posts: {
        posts: [
          {
            slug: 'malformed-content',
            status: 'finalized',
            contentLanguage: null,
            content: POST_CONTENT_FIXTURE,
          },
        ],
      },
    })

    await openStep(user, '글 완성')
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '글의 내용 언어 정보가 없어 내보낼 수 없어요.',
    )
    expect(screen.queryByRole('heading', { name: '내보내기' })).not.toBeInTheDocument()
  })
})

// Plan 11 A12: 용도 is optional, defaults to 없음, and rides the same draft queue as the text.
describe('the post purpose', () => {
  const AUTOSAVED = { timeout: 4_000 }
  const PURPOSES = [
    { id: 'purpose-review', name: '정보성 식당 리뷰', description: '협찬 방문 리뷰' },
    { id: 'purpose-diary', name: '일기' },
  ]
  const POST_PURPOSES = [
    { id: 'purpose-review', name: '정보성 식당 리뷰' },
    { id: 'purpose-diary', name: '일기' },
  ]

  async function pickPurpose(user: ReturnType<typeof userEvent.setup>, name: string) {
    const picker = await screen.findByLabelText('용도')
    await waitFor(() => expect(picker).toBeEnabled())
    const option = await screen.findByRole('option', { name })
    await user.selectOptions(picker, option)
    return picker
  }

  it('defaults a new draft to 없음 and sends no purpose with the create', async () => {
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves },
      purposes: { purposes: PURPOSES },
    })

    const picker = await screen.findByLabelText('용도')
    expect(picker).toHaveValue('')
    expect(within(picker).getByRole('option', { name: '없음' })).toBeInTheDocument()

    const user = userEvent.setup()
    await user.type(screen.getByLabelText('제목'), '제주')
    await waitFor(() => expect(draftSaves).toHaveLength(1), AUTOSAVED)
    // Omitted, not '': the create has no assignment to clear, so the request is byte-for-byte
    // what it was before purposes existed.
    expect(draftSaves[0].purposeId).toBeUndefined()
  })

  it('carries a chosen purpose into the create and links to the management screen', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves, purposes: POST_PURPOSES },
      purposes: { purposes: PURPOSES },
    })

    await pickPurpose(user, '정보성 식당 리뷰')
    // Choosing is not typing: nothing is saved until there is something to save.
    expect(draftSaves).toHaveLength(0)
    expect(screen.getByText('협찬 방문 리뷰')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '용도 관리' })).toHaveAttribute('href', '/purposes')

    await user.type(screen.getByLabelText('제목'), '리뷰 글')
    await waitFor(
      () => expect(draftSaves[0]).toMatchObject({ slug: '', purposeId: 'purpose-review' }),
      AUTOSAVED,
    )
  })

  it('assigns and clears an existing post through the draft queue', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [{ slug: '20260820-jeju', title: '제주' }],
        draftSaves,
        purposes: POST_PURPOSES,
      },
      purposes: { purposes: PURPOSES },
    })

    await pickPurpose(user, '일기')
    // No confirmation sheet: nothing is learned from a purpose, so there is nothing to warn about.
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await waitFor(() =>
      expect(draftSaves[0]).toMatchObject({ slug: '20260820-jeju', purposeId: 'purpose-diary' }),
    )
    await waitFor(() => expect(screen.getByLabelText('용도')).toHaveValue('purpose-diary'))

    await pickPurpose(user, '없음')
    // A present empty string, which is what clears it — distinct from omitting the field.
    await waitFor(() => expect(draftSaves[1]).toMatchObject({ purposeId: '' }))
    await waitFor(() => expect(screen.getByLabelText('용도')).toHaveValue(''))
  })

  // The selection goes out on its own request, so a title save still in flight cannot carry the
  // older assignment over it.
  it('survives a delayed title save', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [{ slug: '20260820-jeju', title: '제주' }],
        draftSaves,
        purposes: POST_PURPOSES,
      },
      purposes: { purposes: PURPOSES },
    })

    await screen.findByLabelText('용도')
    await user.type(screen.getByLabelText('제목'), ' 여행')
    await pickPurpose(user, '일기')

    await waitFor(() => expect(draftSaves.length).toBeGreaterThan(0), AUTOSAVED)
    // Whatever order the requests went out in, the last word on the assignment is the choice.
    const assignments = draftSaves.map((save) => save.purposeId).filter((id) => id !== undefined)
    expect(assignments.at(-1)).toBe('purpose-diary')
    await waitFor(() => expect(screen.getByLabelText('용도')).toHaveValue('purpose-diary'))
  })

  // A failed directory read must not be indistinguishable from "you have no 용도" — the select
  // would be enabled with 없음 alone, and clearing would be the only thing left to do.
  it('says so and offers a retry when the directory cannot be read', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ slug: '20260820-jeju', title: '제주' }] },
      purposes: { listFails: true },
    })

    expect(await screen.findByText(/용도 목록을 불러오지 못했어요/)).toBeInTheDocument()
    expect(screen.getByLabelText('용도')).toBeDisabled()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })

  it('stays usable during a job and says the running one keeps its own brief', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            title: '제주',
            purpose: { id: 'purpose-review', name: '정보성 식당 리뷰' },
            activeJob: { id: 'job-1', kind: 'generate', status: 'running' },
          },
        ],
        purposes: POST_PURPOSES,
      },
      purposes: { purposes: PURPOSES },
      jobs: { jobs: [{ id: 'job-1', kind: 'generate', status: 'running' }] },
    })

    const picker = await screen.findByLabelText('용도')
    await waitFor(() => expect(picker).toHaveValue('purpose-review'))
    expect(picker).toBeEnabled()
    expect(
      await screen.findByText(/진행 중인 AI 작업은 시작할 때의 용도로 끝나요/),
    ).toBeInTheDocument()
  })
})
