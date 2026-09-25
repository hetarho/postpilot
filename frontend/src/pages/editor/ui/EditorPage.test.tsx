import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { discardUploadBatches } from '@/features/upload-photos'
import { ExperimentOrigin, ProtoPlan, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import {
  OBSERVATION_FIXTURE,
  POST_CONTENT_FIXTURE,
  POST_CONTENT_WITH_VIDEO_FIXTURE,
  OBSERVATIONS_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
import type { FakeWriteExperimentStart } from '@/test/experiments'
import type { FakeGenerationStart } from '@/test/jobs'
import {
  FAKE_STORAGE_ORIGIN,
  type FakeDraftSave,
  type FakeOptionsSave,
  type FakePostRow,
  type FakePostsOptions,
} from '@/test/posts'
import type { FakeQualityReading } from '@/test/quality'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

const USER = { id: 'alice' }
const originalClipboard = navigator.clipboard

// This integration file mounts the full routed editor 71 times. Individual cases finish well
// below this bound, but the default 5s becomes flaky while all test files transform in parallel.
vi.setConfig({ testTimeout: 10_000 })

/** The editor is one mounted component showing three step panels, so a test reaches a panel the
 *  post's status did not open by pressing its step. */
async function openStep(user: ReturnType<typeof userEvent.setup>, label: string) {
  await user.click(await screen.findByRole('tab', { name: label }))
}

/** 글 다듬기's two ways out — 확정 and 확정하고 말투 학습 — live behind ONE 확정하기 trigger at the
 *  top-right of the revision row, in a popover that is a bottom sheet on a phone. The panel IS the
 *  confirmation, so a test presses the trigger and then the action it wants. */
async function openFinalize(user: ReturnType<typeof userEvent.setup>) {
  const trigger = await screen.findByRole('button', { name: '확정하기' })
  if (trigger.getAttribute('aria-expanded') !== 'true') await user.click(trigger)
  return screen.findByRole('dialog', { name: '확정하기' })
}

async function finalize(user: ReturnType<typeof userEvent.setup>, action = '확정') {
  const panel = await openFinalize(user)
  await user.click(within(panel).getByRole('button', { name: action }))
}

/** The writing brief — 관찰/작성 모델, 작성 A/B 후보, 목표 언어, 목표 분량 — lives behind ONE trigger
 *  in the dock (change 12), so a test that drives any of them opens it first. 말투 and 템플릿 are the
 *  exceptions and ride the dock's own row; see `dockField` below. */
const BRIEF_TRIGGER = /^(글쓰기 옵션|Writing options)$/
const GENERATE_STEP = /^(글 생성|Generate)$/
const generateTab = () =>
  screen.queryAllByRole('tab').find((tab) => GENERATE_STEP.test(tab.textContent ?? ''))
async function openBrief(user: ReturnType<typeof userEvent.setup>) {
  // Wait for the editor to have its post: the lifecycle bar exists only once it does, and the
  // brief belongs to that bar's first step. `/posts/new` has no bar, so its trigger is already up.
  await waitFor(() =>
    expect(screen.queryByRole('button', { name: BRIEF_TRIGGER }) ?? generateTab()).toBeDefined(),
  )
  const generate = generateTab()
  if (generate && generate.getAttribute('aria-selected') !== 'true') await user.click(generate)
  const trigger = await screen.findByRole('button', { name: BRIEF_TRIGGER })
  if (trigger.getAttribute('aria-expanded') !== 'true') await user.click(trigger)
  return screen.getByRole('dialog', { name: BRIEF_TRIGGER })
}

/** A field inside the brief. Its accessible name is "<label> <current value>" — the WAI-APG
 *  select-only combobox shape — so the label is a prefix, not the whole name. */
async function briefField(user: ReturnType<typeof userEvent.setup>, label: string) {
  await openBrief(user)
  return screen.findByRole('combobox', { name: new RegExp(label) })
}

/** 말투 and 템플릿 are the two parts of the brief that are NOT behind the trigger: both ride the
 *  dock's own row beside the glyph, so a wrong voice or template is visible without opening
 *  anything. */
async function dockField(user: ReturnType<typeof userEvent.setup>, label: RegExp) {
  // Same wait as `openBrief`: the lifecycle bar exists only once the editor has its post, and the
  // dock row these fields ride belongs to that bar's first step.
  await waitFor(() =>
    expect(screen.queryByRole('combobox', { name: label }) ?? generateTab()).toBeDefined(),
  )
  const generate = generateTab()
  if (generate && generate.getAttribute('aria-selected') !== 'true') await user.click(generate)
  return screen.findByRole('combobox', { name: label })
}
const voiceField = (user: ReturnType<typeof userEvent.setup>) => dockField(user, /말투/)
const templateField = (user: ReturnType<typeof userEvent.setup>) => dockField(user, /템플릿/)

afterEach(() => {
  cleanup()
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: originalClipboard })
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
    expect(screen.getByRole('heading', { level: 1, name: '제주 3일' })).toBeInTheDocument()
    expect(screen.getByLabelText('메모')).toHaveValue('첫날은 비')
    expect(screen.getByLabelText('메모')).toHaveClass('bg-field-bg')
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

  it.each([
    {
      locale: 'ko' as const,
      account: { id: 'alice', plan: ProtoPlan.FREE },
      finish: '글 완성',
      exportHeading: '내보내기',
      copyTitle: '제목 복사',
    },
    {
      locale: 'en' as const,
      account: { id: 'root', plan: ProtoPlan.MASTER },
      finish: 'Finish',
      exportHeading: 'Export',
      copyTitle: 'Copy title',
    },
  ])(
    'keeps manual export and makes no publishing call for $locale/$account.plan',
    async ({ locale, account, finish, exportHeading, copyTitle }) => {
      initializeI18n(locale)
      const calls: string[] = []
      const user = userEvent.setup()
      const writeText = vi.fn(async () => undefined)
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: { writeText },
      })
      renderAppAt('/posts/20260820-jeju', {
        user: account,
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

      await openStep(user, finish)
      expect(await screen.findByRole('heading', { name: exportHeading })).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: copyTitle }))
      await waitFor(() => expect(writeText).toHaveBeenCalledWith(POST_CONTENT_FIXTURE.title))
      // The retired publishing ACTION: no control starts one. ③'s 발행 section records an address
      // the owner published by hand, and its heading is not a control.
      expect(
        screen.queryByRole('button', { name: /^(?:발행하기|Publish)$/ }),
      ).not.toBeInTheDocument()
      expect(screen.queryByRole('link', { name: /^(?:발행하기|Publish)$/ })).not.toBeInTheDocument()
      expect(calls.filter((call) => /publish/i.test(call))).toEqual([])
    },
  )

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

    // The stage is NAMED on the page-top status line and its numbers are the bar's value —
    // never spelled out as prose in the dock (change 15).
    expect(await screen.findByText('사진 관찰 중')).toBeInTheDocument()
    const bar = screen.getByRole('progressbar', { name: '작업 진행률' })
    expect(bar).toHaveAttribute('aria-valuenow', '2')
    expect(bar).toHaveAttribute('aria-valuemax', '4')
    expect(screen.queryByText(/사진 2\/4/)).not.toBeInTheDocument()
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
    expect(screen.getByLabelText('수정 요청을 입력하세요')).toBeDisabled()
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
    expect(screen.getByLabelText('수정 요청을 입력하세요')).toBeEnabled()
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
      expect(router.state.location.pathname).toBe('/posts/experiments/experiment-pending'),
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

    const user = userEvent.setup()
    expect(await screen.findByText(/문체 프로필이 비어 있어요/)).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '생성' })).toBeEnabled())
    expect(screen.getByLabelText('글 작업').previousElementSibling).toHaveClass('mt-auto', 'h-6')

    const brief = await openBrief(user)
    expect(within(brief).getByText('사진이 없어 관찰 모델은 필요하지 않아요.')).toBeInTheDocument()
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

    const user = userEvent.setup()
    const generate = await screen.findByRole('button', { name: '생성' })
    const compare = screen.getByRole('button', { name: 'A/B 비교' })
    await waitFor(() => expect(generate).toBeEnabled())
    // Nothing is said under the row, and the button stays live: pressing it is the way to the
    // fix (owner decision 2026-09-25).
    const PAIR = '작성 A/B 모델 두 개를 선택하세요.'
    expect(compare).toBeEnabled()
    expect(screen.queryByText(PAIR)).toBeNull()

    // The press opens the brief on the pair, marked the way a validation error is, and starts
    // nothing. The fix is two dropdowns away rather than a page away.
    await user.click(compare)
    const brief = await screen.findByRole('dialog', { name: '글쓰기 옵션' })
    expect(within(brief).getByText(PAIR)).toBeInTheDocument()
    expect(within(brief).queryByRole('link')).not.toBeInTheDocument()
    for (const label of [/후보 A/, /후보 B/]) {
      const candidate = within(brief).getByRole('combobox', { name: label })
      expect(candidate).toHaveAttribute('aria-invalid', 'true')
      expect(candidate).toHaveAccessibleDescription(expect.stringContaining(PAIR))
    }
    // The ordinary run is not what was refused, so its own field carries no mark.
    expect(within(brief).getByRole('combobox', { name: /^작성 모델/ })).not.toHaveAttribute(
      'aria-invalid',
    )
    await waitFor(() =>
      expect(within(brief).getByRole('combobox', { name: /후보 A/ })).toHaveFocus(),
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

  // Change 21: a post that has already been observed decides what to re-observe BEFORE the
  // enqueue, and confirming the picker untouched reuses everything.
  it('routes generation through the re-observation picker and freezes the confirmed set', async () => {
    const starts: FakeGenerationStart[] = []
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      jobs: { starts },
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            images: POST_IMAGES_FIXTURE,
            observations: OBSERVATIONS_FIXTURE,
          },
        ],
      },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'observer', vision: true },
          { providerId: 'openrouter', modelId: 'writer' },
        ],
        selections: [
          { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'observer' },
          { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
        ],
      },
    })

    const generate = await screen.findByRole('button', { name: '생성' })
    await waitFor(() => expect(generate).toBeEnabled())
    // An unsaved edit, so the draft flush on the start path is a real request whose ORDER
    // against the picker is observable.
    await user.type(screen.getByLabelText('메모'), ' + 최신 내용')
    await user.click(generate)

    // The picker opens instead of enqueueing, and `beforeStart` has NOT run: a cancelled picker
    // must not have forced a draft save.
    const picker = await screen.findByRole('dialog', { name: '다시 관찰할 사진 선택' })
    expect(calls).not.toContain('StartGeneration')
    expect(calls).not.toContain('SavePostDraft')
    await user.click(within(picker).getByRole('button', { name: '취소' }))
    expect(screen.queryByRole('dialog', { name: '다시 관찰할 사진 선택' })).not.toBeInTheDocument()
    expect(calls).not.toContain('StartGeneration')

    // Reopened, one photo checked, confirmed: the frozen set is exactly that photo, and the
    // draft save now sits on the confirm path, before the RPC that consumes it.
    await user.click(screen.getByRole('button', { name: '생성' }))
    await user.click(await screen.findByRole('checkbox', { name: 'IMG_2.jpg 다시 관찰' }))
    await user.click(screen.getByRole('button', { name: '이대로 시작' }))
    await waitFor(() => expect(calls).toContain('StartGeneration'))
    expect(starts).toHaveLength(1)
    expect(starts[0].reobserveFiles).toEqual(['IMG_2.jpg'])
    expect(calls.indexOf('SavePostDraft')).toBeGreaterThanOrEqual(0)
    expect(calls.indexOf('SavePostDraft')).toBeLessThan(calls.indexOf('StartGeneration'))
  })

  it('sends an EMPTY frozen set when the picker is confirmed untouched', async () => {
    const starts: FakeGenerationStart[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      jobs: { starts },
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            images: POST_IMAGES_FIXTURE,
            observations: OBSERVATIONS_FIXTURE,
          },
        ],
      },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'observer', vision: true },
          { providerId: 'openrouter', modelId: 'writer' },
        ],
        selections: [
          { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'observer' },
          { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
        ],
      },
    })

    const generate = await screen.findByRole('button', { name: '생성' })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.click(generate)
    await user.click(await screen.findByRole('button', { name: '이대로 시작' }))

    await waitFor(() => expect(starts).toHaveLength(1))
    // EMPTY, not absent: absent would mean "observe everything" on the wire.
    expect(starts[0].reobserveFiles).toEqual([])
  })

  // A8 (editor half): the A/B comparison shares the picker and the same reuse contract.
  it('routes the A/B comparison through the same picker', async () => {
    const starts: FakeWriteExperimentStart[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      experiments: { starts },
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            images: POST_IMAGES_FIXTURE,
            observations: OBSERVATIONS_FIXTURE,
          },
        ],
      },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'observer', vision: true },
          { providerId: 'openrouter', modelId: 'candidate-a' },
          { providerId: 'openrouter', modelId: 'candidate-b' },
        ],
        selections: [
          { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'observer' },
          { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'candidate-a' },
        ],
        comparisonPairs: [
          {
            stage: Stage.WRITE,
            candidateA: { providerId: 'openrouter', modelId: 'candidate-a' },
            candidateB: { providerId: 'openrouter', modelId: 'candidate-b' },
          },
        ],
      },
    })

    const compare = await screen.findByRole('button', { name: 'A/B 비교' })
    await waitFor(() => expect(compare).toBeEnabled())
    await user.click(compare)
    await user.click(await screen.findByRole('button', { name: '이대로 시작' }))

    await waitFor(() => expect(starts).toHaveLength(1))
    expect(starts[0].reobserveFiles).toEqual([])
  })

  // The picker can sit open long enough for the post to become unstartable. Confirming a stale
  // dialog must not force a draft save or fire an RPC the server is going to refuse.
  it('enqueues nothing when a confirmed picker has gone stale', async () => {
    const starts: FakeGenerationStart[] = []
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      jobs: { starts },
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            images: POST_IMAGES_FIXTURE,
            observations: OBSERVATIONS_FIXTURE,
            pendingExperimentId: 'experiment-1',
          },
        ],
      },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'observer', vision: true },
          { providerId: 'openrouter', modelId: 'writer' },
        ],
        selections: [
          { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'observer' },
          { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
        ],
      },
    })

    // A pending A/B result disables both actions, so the picker never opens and nothing enqueues.
    const generate = await screen.findByRole('button', { name: '생성' })
    await waitFor(() => expect(generate).toBeDisabled())
    await user.click(generate)
    expect(screen.queryByRole('dialog', { name: '다시 관찰할 사진 선택' })).not.toBeInTheDocument()
    expect(calls).not.toContain('StartGeneration')
    expect(calls).not.toContain('SavePostDraft')
    expect(starts).toHaveLength(0)
  })

  // A1/A10 regression: nothing to reuse means no picker, exactly as before change 21.
  it('starts directly when the post has photos but no stored observation', async () => {
    const starts: FakeGenerationStart[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      jobs: { starts },
      posts: { posts: [{ slug: '20260820-jeju', images: POST_IMAGES_FIXTURE }] },
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'observer', vision: true },
          { providerId: 'openrouter', modelId: 'writer' },
        ],
        selections: [
          { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'observer' },
          { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
        ],
      },
    })

    const generate = await screen.findByRole('button', { name: '생성' })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.click(generate)

    await waitFor(() => expect(starts).toHaveLength(1))
    expect(screen.queryByRole('dialog', { name: '다시 관찰할 사진 선택' })).not.toBeInTheDocument()
    expect(starts[0].reobserveFiles).toBeUndefined()
  })

  it('starts an explicit A/B comparison with the configured pair and optional target', async () => {
    const starts: FakeWriteExperimentStart[] = []
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
    const brief = await openBrief(user)
    await user.click(within(brief).getByRole('checkbox', { name: '목표 글자 수 사용' }))
    // The box arrives with the default already in it, so this is a replacement, not an entry.
    await user.clear(within(brief).getByLabelText('목표 글자 수'))
    await user.type(within(brief).getByLabelText('목표 글자 수'), '750')
    await user.click(within(brief).getByRole('button', { name: '저장' }))
    await waitFor(() => expect(calls).toContain('SavePostGenerationOptions'))
    const compare = screen.getByRole('button', { name: 'A/B 비교' })
    await waitFor(() => expect(compare).toBeEnabled())
    await user.click(compare)
    await waitFor(() => expect(calls).toContain('StartWriteExperiment'))
    expect(starts).toEqual([
      {
        postSlug: '20260820-memo',
        // Started from the editor, so its verdict will apply the winner to this very post.
        origin: ExperimentOrigin.EDITOR,
        observeModel: undefined,
        modelA: { providerId: 'openrouter', modelId: 'candidate-a' },
        modelB: { providerId: 'openrouter', modelId: 'candidate-b' },
        targetLength: 750,
      },
    ])
    expect(calls).not.toContain('StartGeneration')
  })

  // A ticked checkbox over a blank number field is an invalid form nobody asked for: the range
  // error renders under a control the user has not touched yet.
  it('fills 목표 글자 수 with a usable default the moment the box is ticked', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      posts: { posts: [{ slug: '20260820-memo' }] },
    })

    const brief = await openBrief(user)
    expect(within(brief).queryByLabelText('목표 글자 수')).not.toBeInTheDocument()

    await user.click(within(brief).getByRole('checkbox', { name: '목표 글자 수 사용' }))
    const field = within(brief).getByLabelText('목표 글자 수')
    expect(field).toHaveValue(1000)
    expect(field).not.toHaveAttribute('aria-invalid')
    expect(within(brief).getByRole('button', { name: '저장' })).toBeEnabled()

    // What the user typed outranks the default, so unticking and reticking never loses it.
    await user.clear(field)
    await user.type(field, '2400')
    await user.click(within(brief).getByRole('checkbox', { name: '목표 글자 수 사용' }))
    await user.click(within(brief).getByRole('checkbox', { name: '목표 글자 수 사용' }))
    expect(within(brief).getByLabelText('목표 글자 수')).toHaveValue(2400)
  })

  it('restores and explicitly clears a stored target length without starting generation', async () => {
    const calls: string[] = []
    const optionSaves: FakeOptionsSave[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: {
        posts: [{ slug: '20260820-memo', targetLength: 1200, tagCount: 6, useMemory: true }],
        optionSaves,
      },
    })

    const dialog = await openBrief(user)
    expect(within(dialog).getByRole('checkbox', { name: '목표 글자 수 사용' })).toBeChecked()
    expect(within(dialog).getByLabelText('목표 글자 수')).toHaveValue(1200)
    expect(calls).not.toContain('SavePostGenerationOptions')

    await user.click(within(dialog).getByRole('checkbox', { name: '목표 글자 수 사용' }))
    await user.click(within(dialog).getByRole('button', { name: '저장' }))
    // One whole set: natural length, and the post's other four as they stand.
    await waitFor(() =>
      expect(optionSaves).toEqual([
        {
          slug: '20260820-memo',
          targetLength: undefined,
          tagCount: 6,
          useMemory: true,
          qualityRules: [],
          field: '',
        },
      ]),
    )
    expect(calls).not.toContain('StartGeneration')
    expect(calls).not.toContain('StartWriteExperiment')
  })

  // A8 (client half): 확정 copies the AI title into `posts.title`, and the editor still holds the
  // 가제 in state where `useAutosave` would write it straight back on the next keystroke.
  it('re-seeds the local 가제 from the confirmed title before another save can queue', async () => {
    const draftSaves: FakeDraftSave[] = []
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-final', {
      user: USER,
      calls,
      posts: {
        draftSaves,
        posts: [
          {
            slug: '20260820-final',
            status: 'review',
            title: '가제',
            content: POST_CONTENT_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
          },
        ],
      },
    })

    await finalize(user)

    await waitFor(() => expect(calls).toContain('FinalizePost'))
    await openStep(user, '글 생성')
    await waitFor(() =>
      expect(screen.getByLabelText('제목')).toHaveValue(POST_CONTENT_FIXTURE.title),
    )
    // The next ordinary save must carry the confirmed title, not the placeholder — which is what
    // the list row is read from (A8/A9).
    await user.type(screen.getByLabelText('메모'), '뒷이야기')
    await waitFor(() => expect(draftSaves).toHaveLength(1), { timeout: 4_000 })
    await user.click(screen.getByRole('link', { name: '글 목록' }))
    expect(
      await screen.findByRole('link', { name: new RegExp(POST_CONTENT_FIXTURE.title) }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /가제/ })).not.toBeInTheDocument()
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
    // 확정하기 ends 글 다듬기, where the post opens, and offers both ways out.
    const panel = await openFinalize(user)
    const only = within(panel).getByRole('button', { name: '확정' })
    expect(only).toBeEnabled()
    expect(within(panel).getByRole('button', { name: '확정하고 말투 학습' })).toBeDisabled()
    await user.click(only)
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
    const panel = await openFinalize(user)
    const combined = within(panel).getByRole('button', { name: '확정하고 말투 학습' })
    await waitFor(() => expect(combined).toBeEnabled())
    await user.click(combined)
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
    expect(await screen.findByRole('button', { name: '확정하기' })).toBeInTheDocument()

    await openStep(user, '글 완성')
    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()
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
    expect(screen.queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()
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

  // A5. Someone else's slug is 403, not 404 (spec/legacy/policy/posts.md).
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

// Real timers on template: these walk the whole flow through the router, and
// @testing-library's async helpers look for jest's fake-timer API, so vitest's is
// invisible to them and every `waitFor` would spin on a clock nothing advances. The
// debounce window itself is covered by features/save-draft's own tests.
describe('the delete control', () => {
  // Job 40 A1: an existing post can be deleted from its own editor.
  it('offers the delete trigger on a saved post', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ slug: '20260820-jeju', title: '제주 3일' }] },
    })

    expect(await screen.findByRole('button', { name: '글 삭제하기' })).toBeInTheDocument()
  })

  // A draft with no slug has nothing to delete, so the control is absent rather than disabled.
  it('offers nothing on /posts/new', async () => {
    renderAppAt('/posts/new', { user: USER })

    expect(await screen.findByLabelText('제목')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '글 삭제하기' })).not.toBeInTheDocument()
  })
})

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
      await screen.findByLabelText('사진·영상 추가'),
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

    await user.upload(await screen.findByLabelText('사진·영상 추가'), new File(['x'], 'IMG_1.jpg'))

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

    await user.upload(await screen.findByLabelText('사진·영상 추가'), new File(['x'], 'setup.exe'))

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

    // ② is where a review post opens: the draft, and the dock that carries the revision and both
    // confirmations. No 가제, no writing brief, no generation control.
    expect(await screen.findByRole('heading', { name: '글 다듬기' })).toBeInTheDocument()
    expect(screen.queryByLabelText('제목')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /옵션/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '생성' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '내보내기' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('수정 요청을 입력하세요')).toBeInTheDocument()
    const ways = await openFinalize(user)
    expect(within(ways).getByRole('button', { name: '확정' })).toBeInTheDocument()
    expect(within(ways).getByRole('button', { name: '확정하고 말투 학습' })).toBeInTheDocument()
    await user.keyboard('{Escape}')

    await openStep(user, '글 생성')
    expect(await screen.findByLabelText('제목')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '글 다듬기' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('수정 요청을 입력하세요')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()
    const brief = await openBrief(user)
    expect(within(brief).getByRole('combobox', { name: /후보 A/ })).toBeInTheDocument()
    await user.keyboard('{Escape}')

    await openStep(user, '글 완성')
    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '말투 학습' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('제목')).not.toBeInTheDocument()
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

    await screen.findByLabelText('제목')
    calls.length = 0

    await openStep(user, '글 다듬기')
    expect(await screen.findByText(/아직 다듬을 글이 없어요/)).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: '글 다듬기' })).toBeEnabled()

    await user.click(screen.getByRole('button', { name: '글 생성으로 가기' }))
    expect(await screen.findByLabelText('제목')).toBeInTheDocument()

    // Opening a step is not an action: no status change, no job, no provider call.
    expect(calls).not.toContain('SavePostDraft')
    expect(calls).not.toContain('GeneratePost')
    expect(calls).not.toContain('SavePostContent')
  })

  // Change 05 A6: steps are panels, so a save started before a step change still completes.
  // The 가제 now belongs to 글 생성 alone (change 12), so this also proves that unmounting the
  // FIELD cannot strand the queued save — the value and its queue live above the panels.
  it('completes a title save started before the step changed', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      calls,
      posts: { posts: [{ ...reviewPost, title: '제주 3일' }] },
    })

    await openStep(user, '글 생성')
    const title = await screen.findByLabelText('제목')
    await user.type(title, ' 여행기')
    await openStep(user, '글 완성')
    expect(screen.queryByLabelText('제목')).not.toBeInTheDocument()

    await waitFor(() => expect(calls).toContain('SavePostDraft'), { timeout: 4_000 })
    await openStep(user, '글 생성')
    expect(await screen.findByLabelText('제목')).toHaveValue('제주 3일 여행기')
  })

  // A10/A14, amending change 05 A11: ① and ② both always dock — 생성 ends the first step and
  // 확정 the second — while ③ still docks only when there is something to report. There is
  // exactly ONE bar in the scroller on every step (§4.3).
  it('docks the step-ending actions on ① and ②, and nothing on a quiet ③', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, canFinalize: true }] },
      // ① only offers 생성 once the models it needs have been chosen — this post has photos, so
      // that is a vision 관찰 model as well as a 작성 model. With none of them chosen the bar
      // renders the way to go and choose them instead, which is a different test.
      providers: {
        models: [
          { providerId: 'openrouter', modelId: 'seer', vision: true },
          { providerId: 'openrouter', modelId: 'writer' },
        ],
        selections: [
          { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'seer' },
          { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
        ],
      },
    })

    const dock = await screen.findByLabelText('글 작업')
    expect(screen.getAllByLabelText('글 작업')).toHaveLength(1)
    // ONE surface: the revision instruction with its icon send button, and 확정하기 in its
    // heading. Neither section is rendered in the panel any more.
    expect(within(dock).getByLabelText('수정 요청을 입력하세요')).toBeInTheDocument()
    expect(within(dock).getByRole('button', { name: '수정' })).toBeInTheDocument()
    expect(within(dock).getByRole('button', { name: '확정하기' })).toBeInTheDocument()

    await openStep(user, '글 생성')
    const generateDock = await screen.findByLabelText('글 작업')
    expect(screen.getAllByLabelText('글 작업')).toHaveLength(1)
    expect(within(generateDock).getByRole('button', { name: '생성' })).toBeInTheDocument()
    expect(within(generateDock).getByRole('button', { name: /옵션/ })).toBeInTheDocument()
    expect(within(generateDock).queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()

    await openStep(user, '글 완성')
    expect(await screen.findByRole('button', { name: '말투 학습' })).toBeInTheDocument()
    expect(screen.queryByLabelText('글 작업')).not.toBeInTheDocument()
  })

  // A step whose models were never chosen keeps its two actions: a press is what takes the user to
  // the brief, with what that run is missing marked there (owner decision 2026-09-25).
  it('opens the brief on the missing model when a press finds nothing chosen', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, status: 'draft', images: [] }] },
    })

    const WRITE = '활성 작성 모델을 선택하세요.'
    const dock = await screen.findByLabelText('글 작업')
    const generate = await within(dock).findByRole('button', { name: '생성' })
    await waitFor(() => expect(generate).toBeEnabled())
    expect(within(dock).getByRole('button', { name: 'A/B 비교' })).toBeEnabled()
    // Nothing is written under the row, and there is no second route out of the bar.
    expect(within(dock).queryByText(WRITE)).not.toBeInTheDocument()
    expect(within(dock).queryByRole('link')).not.toBeInTheDocument()

    await user.click(generate)
    const brief = await screen.findByRole('dialog', { name: '글쓰기 옵션' })
    const writer = within(brief).getByRole('combobox', { name: /^작성 모델/ })
    expect(within(brief).getByText(WRITE)).toBeInTheDocument()
    expect(writer).toHaveAttribute('aria-invalid', 'true')
    expect(writer).toHaveAccessibleDescription(expect.stringContaining(WRITE))
    // It shakes once, and focus lands on it.
    expect(writer.closest('.animate-shake')).not.toBeNull()
    await waitFor(() => expect(writer).toHaveFocus())
    // A post with no photo needs no observe model, and 생성 asks nothing of the A/B pair.
    expect(within(brief).getByRole('combobox', { name: /^관찰 모델/ })).not.toHaveAttribute(
      'aria-invalid',
    )
    expect(within(brief).queryByText('작성 A/B 모델 두 개를 선택하세요.')).toBeNull()

    // The marks answer that press: closed and reopened from its own glyph, the brief is plain.
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '글쓰기 옵션' })).toBeNull())
    const reopened = await openBrief(user)
    expect(within(reopened).queryByText(WRITE)).toBeNull()
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

  // The step bar is the first thing on the screen, above the 가제 that 글 생성 now owns.
  it('puts the step bar above the title', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, status: 'draft' }] },
    })

    const tab = await screen.findByRole('tab', { name: '글 생성' })
    const title = await screen.findByLabelText('제목')
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

  // VIDEO-2: a VIDEO block carries the IMAGE fields and none of its own, so the same three
  // controls edit it — only the list it picks from differs, and it is offered only when the
  // post actually has a clip.
  it('edits a VIDEO block with the attached-video list, alt and caption', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-video', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-video',
            status: 'review',
            content: POST_CONTENT_WITH_VIDEO_FIXTURE,
            images: POST_IMAGES_FIXTURE,
            videos: [{ id: 'video-1', filename: 'clip.mp4' }],
            contentRevision: 1n,
            machineBaselineRevision: 1n,
          },
        ],
      },
    })

    const blockIndex = POST_CONTENT_WITH_VIDEO_FIXTURE.blocks.length - 1
    await user.click(await screen.findByRole('button', { name: `${blockIndex + 1}번째 블록 수정` }))
    expect(screen.getByText('첨부 영상')).toBeInTheDocument()
    const caption = screen.getByPlaceholderText('캡션 (선택)')
    await user.type(caption, '파도')
    await user.click(screen.getByRole('button', { name: '저장' }))
    expect(await screen.findByText('파도')).toBeInTheDocument()
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

    await finalize(user)

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
    const picker = await voiceField(user)
    await waitFor(() => expect(picker).toBeEnabled())
    await user.click(picker)
    await user.click(
      await screen.findByRole('option', {
        name: voiceId === 'voice-review' ? '리뷰' : '기본 말투',
      }),
    )
    return picker
  }
  const confirmDialog = () => screen.findByRole('dialog', { name: '말투를 바꿀까요?' })
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

    expect(await voiceField(user)).toHaveTextContent('기본 말투')
    await user.type(screen.getByLabelText('제목'), '제주')

    await waitFor(
      () => expect(router.state.location.pathname).toBe('/posts/20260828-제주'),
      AUTOSAVED,
    )
    expect(draftSaves[0]).toEqual({
      slug: '',
      voiceId: 'voice-default',
      templateId: undefined,
      templateAnswers: [],
      targetLanguage: 'ko',
    })
    // The editor the mint mounted shows the same voice, and later saves leave it alone.
    expect(await voiceField(user)).toHaveTextContent('기본 말투')
    await user.type(screen.getByLabelText('메모'), '첫날')
    await waitFor(() => expect(draftSaves).toHaveLength(2), AUTOSAVED)
    expect(draftSaves[1]).toEqual({
      slug: '20260828-제주',
      voiceId: undefined,
      templateId: undefined,
      templateAnswers: [],
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
          templateId: undefined,
          templateAnswers: [],
          targetLanguage: 'ko',
        }),
      AUTOSAVED,
    )
    await waitFor(() => expect(router.state.location.pathname).toBe('/posts/20260828-리뷰-글'))
    expect(await voiceField(user)).toHaveTextContent('리뷰')
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

    const picker = await voiceField(user)
    await waitFor(() => expect(picker).toHaveTextContent('기본 말투'))
    await pickVoice(user, 'voice-review')
    const dialog = await confirmDialog()
    expect(dialog).toHaveTextContent('지금까지 배운 내용은 이전 말투에 남고')
    // The choice is not applied until it is confirmed.
    expect(draftSaves).toHaveLength(0)
    await user.click(within(dialog).getByRole('button', { name: '말투 변경' }))

    await waitFor(() =>
      expect(draftSaves).toEqual([
        { slug: '20260820-jeju', voiceId: 'voice-review', templateAnswers: [] },
      ]),
    )
    await waitFor(() => expect(picker).toHaveTextContent('리뷰'))
    await user.keyboard('{Escape}')
    // The canonical content survived; learning needs a new machine result first. Both live in
    // 글 다듬기's dock, so the step has to be the one that owns them.
    await openStep(user, '글 다듬기')
    const ways = await openFinalize(user)
    expect(within(ways).getByRole('button', { name: '확정' })).toBeEnabled()
    expect(within(ways).getByRole('button', { name: '확정하고 말투 학습' })).toBeDisabled()
    await user.keyboard('{Escape}')
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
    await user.click(within(await confirmDialog()).getByRole('button', { name: '말투 변경' }))
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
    await user.click(within(await confirmDialog()).getByRole('button', { name: '취소' }))

    expect(screen.queryByRole('dialog', { name: '말투를 바꿀까요?' })).not.toBeInTheDocument()
    expect(await voiceField(user)).toHaveTextContent('기본 말투')
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

    const user = userEvent.setup()
    expect(await voiceField(user)).toBeDisabled()
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
    await user.click(within(await confirmDialog()).getByRole('button', { name: '말투 변경' }))

    // `findAllByRole`: the dock's own save notice can be an alert at the same moment.
    const alerts = await screen.findAllByRole('alert')
    expect(alerts.map((alert) => alert.textContent).join('\n')).toContain(
      '고른 말투를 찾을 수 없어요',
    )
    const picker = await voiceField(user)
    expect(picker).toHaveTextContent('기본 말투')
    expect(picker).toBeEnabled()
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

    // Named twice on template, and both are on the page from the first paint now that the picker
    // rides the dock: the picker's closed trigger (the disabled current option) and 글 생성's own
    // warning.
    expect(await screen.findAllByText('삭제된 말투 · 옛 말투')).toHaveLength(2)
    const picker = await voiceField(user)
    expect(picker).toHaveTextContent('삭제된 말투 · 옛 말투')
    // The refusal is said ONCE, by the surface that carries the way out: 글 생성's tombstone
    // warning offers 복원, so the dock only disables the action rather than re-writing the reason
    // under it (change 15).
    await waitFor(() => expect(screen.getByRole('button', { name: '생성' })).toBeDisabled())
    expect(
      screen.queryByText('생성: 삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.'),
    ).not.toBeInTheDocument()
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

    // Twice from the first paint: the dock's picker names the tombstone, and 글 생성 warns about it.
    await screen.findAllByText('삭제된 말투 · 옛 말투')
    await pickVoice(user, 'voice-review')
    await user.click(within(await confirmDialog()).getByRole('button', { name: '말투 변경' }))

    await waitFor(() =>
      expect(draftSaves).toEqual([
        { slug: '20260820-jeju', voiceId: 'voice-review', templateAnswers: [] },
      ]),
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

    expect(await briefField(user, 'Post language')).toHaveTextContent('English')
    await user.keyboard('{Escape}')
    await user.type(screen.getByLabelText('Title'), 'English draft')

    await waitFor(() => expect(draftSaves[0]?.targetLanguage).toBe('en'), AUTOSAVED)
  })

  it('sends an explicit pre-create language instead of the UI-locale default', async () => {
    initializeI18n('en')
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })

    const language = await briefField(user, 'Post language')
    await user.click(language)
    await user.click(await screen.findByRole('option', { name: 'Korean' }))
    await user.keyboard('{Escape}')
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

    const user = userEvent.setup()
    expect(await briefField(user, 'Post language')).toHaveTextContent('Korean')
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

// Plan 11 A12: 템플릿 is optional, defaults to 없음, and rides the same draft queue as the text.
describe('the post template', () => {
  const AUTOSAVED = { timeout: 4_000 }
  const PURPOSES = [
    { id: 'template-review', name: '정보성 식당 리뷰', description: '협찬 방문 리뷰' },
    { id: 'template-diary', name: '일기' },
  ]
  const POST_PURPOSES = [
    { id: 'template-review', name: '정보성 식당 리뷰' },
    { id: 'template-diary', name: '일기' },
  ]

  async function pickTemplate(user: ReturnType<typeof userEvent.setup>, name: string) {
    const picker = await templateField(user)
    await waitFor(() => expect(picker).toBeEnabled())
    await user.click(picker)
    await user.click(await screen.findByRole('option', { name }))
    return picker
  }

  it('defaults a new draft to 없음 and sends no template with the create', async () => {
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves },
      templates: { templates: PURPOSES },
    })

    const user = userEvent.setup()
    const picker = await templateField(user)
    expect(picker).toHaveTextContent('없음')
    await user.click(picker)
    expect(screen.getByRole('option', { name: '없음', selected: true })).toBeInTheDocument()
    await user.keyboard('{Escape}')

    await user.keyboard('{Escape}')
    await user.type(screen.getByLabelText('제목'), '제주')
    await waitFor(() => expect(draftSaves).toHaveLength(1), AUTOSAVED)
    // Omitted, not '': the create has no assignment to clear, so the request is byte-for-byte
    // what it was before templates existed.
    expect(draftSaves[0].templateId).toBeUndefined()
  })

  // TEMPLATE-48: picking a template seeds the post's two generation options, and the screen
  // shows the seeded values at once — on the response the picker already awaits, with no
  // second call and nothing saying a template wrote them.
  it('shows the numbers a picked template seeded', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/posts/20260820-memo', {
      user: USER,
      calls,
      posts: {
        posts: [{ slug: '20260820-memo' }],
        templates: [
          { id: 'template-review', name: '정보성 식당 리뷰', targetLength: 1800, tagCount: 7 },
          { id: 'template-diary', name: '일기' },
        ],
      },
      templates: { templates: PURPOSES },
    })

    const brief = await openBrief(user)
    // The post starts with natural length and the default count.
    expect(within(brief).queryByLabelText('목표 글자 수')).not.toBeInTheDocument()
    expect(within(brief).getByLabelText('태그 개수')).toHaveValue(4)
    await user.keyboard('{Escape}')

    await pickTemplate(user, '정보성 식당 리뷰')
    await waitFor(() => expect(calls).toContain('SavePostDraft'), AUTOSAVED)

    const seeded = await openBrief(user)
    // The length arrived, so its tick is on and the field carries it.
    await waitFor(() => expect(within(seeded).getByLabelText('목표 글자 수')).toHaveValue(1800))
    expect(within(seeded).getByLabelText('태그 개수')).toHaveValue(7)
    // Seeded, not saved twice: the values rode the assignment's own response.
    expect(calls).not.toContain('SavePostGenerationOptions')
  })

  it('carries a chosen template into the create', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves, templates: POST_PURPOSES },
      templates: { templates: PURPOSES },
    })

    await pickTemplate(user, '정보성 식당 리뷰')
    // Choosing is not typing: nothing is saved until there is something to save.
    expect(draftSaves).toHaveLength(0)
    // The dock row is three controls wide on a 360px screen, so the chosen 템플릿's own brief and
    // the way to the 템플릿 page are not on it: both belong to the directory that owns them.
    expect(screen.queryByText('협찬 방문 리뷰')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '템플릿 관리' })).not.toBeInTheDocument()

    await user.type(screen.getByLabelText('제목'), '리뷰 글')
    await waitFor(
      () => expect(draftSaves[0]).toMatchObject({ slug: '', templateId: 'template-review' }),
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
        templates: POST_PURPOSES,
      },
      templates: { templates: PURPOSES },
    })

    const picker = await pickTemplate(user, '일기')
    // No confirmation sheet: nothing is learned from a template, so there is nothing to warn about.
    expect(screen.queryByRole('dialog', { name: '말투를 바꿀까요?' })).not.toBeInTheDocument()
    await waitFor(() =>
      expect(draftSaves[0]).toMatchObject({ slug: '20260820-jeju', templateId: 'template-diary' }),
    )
    await waitFor(() => expect(picker).toHaveTextContent('일기'))

    await pickTemplate(user, '없음')
    // A present empty string, which is what clears it — distinct from omitting the field.
    await waitFor(() => expect(draftSaves[1]).toMatchObject({ templateId: '' }))
    await waitFor(() => expect(picker).toHaveTextContent('없음'))
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
        templates: POST_PURPOSES,
      },
      templates: { templates: PURPOSES },
    })

    await user.type(await screen.findByLabelText('제목'), ' 여행')
    const picker = await pickTemplate(user, '일기')

    await waitFor(() => expect(draftSaves.length).toBeGreaterThan(0), AUTOSAVED)
    // Whatever order the requests went out in, the last word on the assignment is the choice.
    const assignments = draftSaves.map((save) => save.templateId).filter((id) => id !== undefined)
    expect(assignments.at(-1)).toBe('template-diary')
    await waitFor(() => expect(picker).toHaveTextContent('일기'))
  })

  // A failed directory read must not be indistinguishable from "you have no 템플릿" — the select
  // would be enabled with 없음 alone, and clearing would be the only thing left to do.
  it('says so and offers a retry when the directory cannot be read', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ slug: '20260820-jeju', title: '제주' }] },
      templates: { listFails: true },
    })

    const user = userEvent.setup()
    const picker = await templateField(user)
    expect(await screen.findByText(/템플릿 목록을 불러오지 못했어요/)).toBeInTheDocument()
    expect(picker).toBeDisabled()
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
            template: { id: 'template-review', name: '정보성 식당 리뷰' },
            activeJob: { id: 'job-1', kind: 'generate', status: 'running' },
          },
        ],
        templates: POST_PURPOSES,
      },
      templates: { templates: PURPOSES },
      jobs: { jobs: [{ id: 'job-1', kind: 'generate', status: 'running' }] },
    })

    const user = userEvent.setup()
    const picker = await templateField(user)
    await waitFor(() => expect(picker).toHaveTextContent('정보성 식당 리뷰'))
    expect(picker).toBeEnabled()
    expect(
      await screen.findByText(/진행 중인 AI 작업은 시작할 때의 템플릿으로 끝나요/),
    ).toBeInTheDocument()
  })
})

// Change 15: everything the editor has to SAY about its own state is one 2px bar plus one line at
// the top of the page, and the dock below it holds only controls and the reason one is refused.
describe('the editor status region', () => {
  const statusLine = () => screen.getByRole('status', { name: '글 상태' })

  it('reports the post status on the line when nothing else is happening', async () => {
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
            finalizedRevision: 1n,
            canFinalize: true,
          },
        ],
      },
    })

    // A7: the status IS the report. The 이 revision을 확정했어요 notice that used to stand on
    // 글 완성 said the same thing as an event that nothing ever took down.
    await waitFor(() => expect(statusLine()).toHaveTextContent('확정'))
    expect(screen.queryByText('이 revision을 확정했어요.')).not.toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  // A2: 작업 준비 중 is not a stage and is never shown; a job with no reportable ratio is the same
  // 2px track without a value.
  it('renders the top bar indeterminate for a stage that reports no ratio', async () => {
    const active = { id: 'job-1', kind: 'generate', status: 'running', stage: 'write' }
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ slug: '20260820-jeju', activeJob: active }] },
      jobs: { jobs: [active] },
    })

    const bar = await screen.findByRole('progressbar', { name: '작업 진행률' })
    expect(bar).not.toHaveAttribute('aria-valuenow')
    await waitFor(() => expect(statusLine()).toHaveTextContent('작성 중'))
    expect(screen.queryByText('작업 준비 중')).not.toBeInTheDocument()
  })

  // A4: the line goes quiet on its own, so the status can get back onto it.
  it('shows 저장됨 for the settle interval and then returns the line to the status', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ slug: '20260820-jeju', status: 'draft', title: '제주' }] },
    })

    await user.type(await screen.findByLabelText('제목'), ' 여행')
    await waitFor(() => expect(statusLine()).toHaveTextContent('저장됨'), { timeout: 5_000 })
    await waitFor(() => expect(statusLine()).toHaveTextContent('초안'), { timeout: 5_000 })
    expect(statusLine()).not.toHaveTextContent('저장됨')
  })

  // A5: one statement of what to do, and it is the control that resolves it.
  it('leaves the A/B way out as the only word on a pending comparison', async () => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        posts: [
          {
            slug: '20260820-jeju',
            status: 'draft',
            images: [],
            pendingExperimentId: 'experiment-pending',
          },
        ],
      },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'writer' }],
        selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      },
    })

    expect(await screen.findByRole('link', { name: 'A/B 결과 확인' })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '생성' })).toBeDisabled())
    expect(screen.getByRole('button', { name: 'A/B 비교' })).toBeDisabled()
    expect(screen.queryByText('먼저 대기 중인 A/B 결과를 확인해 주세요.')).not.toBeInTheDocument()
    expect(screen.queryByText(/^생성:/)).not.toBeInTheDocument()
  })
})

// Change 16 A14: the server requires a COMPLETED voice-learning event before it accepts sentence
// feedback, and a post on 글 다듬기 is in `review` — never finalized, never learned. The control
// therefore moved to 글 완성 and is gated on the same condition the server enforces.
describe('sentence feedback', () => {
  const learnedPost = {
    slug: '20260820-final',
    status: 'finalized',
    content: POST_CONTENT_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
    finalizedRevision: 1n,
    finalizedAt: '2026-08-20T12:00:00Z',
  }

  it('is absent on 글 다듬기 and present on 글 완성 once the learning run has completed', async () => {
    const user = userEvent.setup()
    const key = 'postpilot:voice-learning:alice:20260820-final'
    const stored = new Map<string, string>()
    vi.stubGlobal('localStorage', {
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
      posts: { posts: [learnedPost] },
      jobs: { jobs: [{ id: 'learn-1', kind: 'voice_learn', status: 'done' }] },
      providers: {
        models: [{ providerId: 'openrouter', modelId: 'analyzer' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' }],
      },
    })

    expect(await screen.findByText('이 글에서 말투를 배웠어요.')).toBeInTheDocument()
    const feedback = screen.getByRole('button', { name: '문장 의견' })

    // What it SAYS: it teaches the voice, it does not change the post, and it names the thing
    // that does change the post.
    await user.click(feedback)
    const dialog = await screen.findByRole('dialog', { name: '어떤 점을 바꾸고 싶나요?' })
    expect(dialog).toHaveTextContent('이 의견은 말투를 가르칩니다. 이 글은 바뀌지 않아요.')
    expect(dialog).toHaveTextContent('AI 수정')
    expect(dialog).not.toHaveTextContent('이 반응만으로 새 규칙이 생기거나 활성화되지는 않습니다.')

    await user.click(within(dialog).getByRole('button', { name: '취소' }))
    await openStep(user, '글 다듬기')
    await screen.findByRole('button', { name: '제목과 요약, 태그 수정' })
    expect(screen.queryByRole('button', { name: '문장 의견' })).not.toBeInTheDocument()
    localStorage.removeItem(key)
  })

  it('is not offered on 글 완성 for a post whose learning run has not completed', async () => {
    renderAppAt('/posts/20260820-final', {
      user: USER,
      posts: { posts: [learnedPost] },
    })
    await screen.findByRole('button', { name: '말투 학습' })
    expect(screen.queryByRole('button', { name: '문장 의견' })).not.toBeInTheDocument()
  })
})

// POST-54 · POST-62: the selected template's data fields belong to ①, under the memo, because
// they are the material 글 생성 works from. They autosave on the memo's own queue.
describe('the template data fields in ①', () => {
  const AUTOSAVED = { timeout: 4_000 }
  const WITH_FIELDS = [
    {
      id: 'template-review',
      name: '정보성 식당 리뷰',
      body: '<write>인트로</write>\n<ask label="방문일"/>\n<ask label="총평 별점">별점과 총평</ask>',
    },
  ]

  it('seeds the fields from what the post already answered', async () => {
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260301-jeju', {
      user: USER,
      posts: {
        draftSaves,
        posts: [
          {
            slug: '20260301-jeju',
            title: '제주',
            memo: '갔다',
            template: { id: 'template-review', name: '정보성 식당 리뷰' },
            templateAnswers: [{ label: '총평 별점', text: '4.5점', enabled: false }],
          },
        ],
        templates: [{ id: 'template-review', name: '정보성 식당 리뷰' }],
      },
      templates: { templates: WITH_FIELDS },
    })

    // Both fields, in body order, with the stored answer and switch on the second.
    expect(await screen.findByLabelText('방문일')).toHaveValue('')
    const rated = screen.getByLabelText('총평 별점')
    expect(rated).toHaveValue('4.5점')
    // Off keeps its text and greys the field: the switch is a decision the author can take back.
    expect(rated).toBeDisabled()
    expect(screen.getByRole('switch', { name: '총평 별점 넣기' })).not.toBeChecked()
    // A mount is not an edit: the fields the post has not answered read as their default, so
    // nothing is queued and no save goes out.
    expect(draftSaves).toEqual([])
  })

  it('autosaves an answer on the memo’s own queue', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/20260301-jeju', {
      user: USER,
      posts: {
        draftSaves,
        posts: [
          {
            slug: '20260301-jeju',
            title: '제주',
            memo: '갔다',
            template: { id: 'template-review', name: '정보성 식당 리뷰' },
          },
        ],
        templates: [{ id: 'template-review', name: '정보성 식당 리뷰' }],
      },
      templates: { templates: WITH_FIELDS },
    })

    await user.type(await screen.findByLabelText('방문일'), '2026-03-01')
    await waitFor(() => expect(draftSaves.length).toBeGreaterThan(0), AUTOSAVED)
    const last = draftSaves[draftSaves.length - 1]
    // The whole set on screen rides one save; every entry is an upsert of its own label.
    expect(last.templateAnswers).toEqual([
      { label: '방문일', text: '2026-03-01', enabled: true },
      { label: '총평 별점', text: '', enabled: true },
    ])
  })

  it('renders no field section for a post on 없음', async () => {
    renderAppAt('/posts/20260301-jeju', {
      user: USER,
      posts: { posts: [{ slug: '20260301-jeju', title: '제주', memo: '갔다' }] },
      templates: { templates: WITH_FIELDS },
    })

    expect(await screen.findByLabelText('메모')).toBeInTheDocument()
    expect(screen.queryByText('템플릿 입력란')).not.toBeInTheDocument()
  })
})

/** The account's M1 over band, so the brief's quality rows offer a tick. */
const OVER: FakeQualityReading[] = [
  {
    metric: 'title_saturation',
    verdict: 'over_band',
    minimum: 10,
    publishedCount: 12,
    ruleText: '제목마다 “성수 카페”를 반복하지 마세요.',
    values: { share: 0.42, shareWarnAbove: 0.3 },
  },
]

// POST-89: 분야 and 기억 사용 are run options, so they live in the writing brief with the other
// three and save together by its 저장 — none of them is ①'s material (POST-54).
describe('the brief run options', () => {
  const SLUG = '20260301-jeju'
  const WITH_FIELDS = [
    {
      id: 'template-review',
      name: '정보성 식당 리뷰',
      body: '<write>인트로</write>\n<ask label="방문일"/>\n<ask label="총평 별점">별점과 총평</ask>',
    },
  ]
  const POST_TEMPLATES = [{ id: 'template-review', name: '정보성 식당 리뷰' }]
  const following = (first: Node, second: Node) =>
    Boolean(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING)
  const m1 = () => screen.findByRole('checkbox', { name: /^제목 도배율 42%/ })
  const briefClosed = () =>
    waitFor(() =>
      expect(screen.getByRole('button', { name: BRIEF_TRIGGER })).toHaveAttribute(
        'aria-expanded',
        'false',
      ),
    )

  it('keeps 분야 and 기억 사용 out of ①, after the quality rows in the brief', async () => {
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [
          {
            slug: SLUG,
            title: '제주',
            template: { id: 'template-review', name: '정보성 식당 리뷰' },
          },
        ],
        templates: POST_TEMPLATES,
      },
      templates: { templates: WITH_FIELDS },
      quality: { accounts: { [SLUG]: OVER } },
    })

    // ① holds the post's material, and 분야 and 기억 사용 are not part of it.
    expect(await screen.findByLabelText('총평 별점')).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: '분야' })).toBeNull()
    expect(screen.queryByRole('checkbox', { name: '기억 사용' })).toBeNull()

    const brief = await openBrief(user)
    const tags = within(brief).getByLabelText('태그 개수')
    const quality = await within(brief).findByText('발행 글 점검')
    const field = within(brief).getByRole('group', { name: '분야' })
    const memory = within(brief).getByRole('checkbox', { name: '기억 사용' })
    const save = within(brief).getByRole('button', { name: '저장' })
    expect(following(tags, quality)).toBe(true)
    expect(following(quality, field)).toBe(true)
    expect(following(field, memory)).toBe(true)
    expect(following(memory, save)).toBe(true)
  })

  it('saves the brief’s run options together and keeps them across a reload', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const optionSaves: FakeOptionsSave[] = []
    const first = renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: { calls, optionSaves, posts: [{ slug: SLUG, title: '제주' }] },
      quality: { accounts: { [SLUG]: OVER } },
    })

    const brief = await openBrief(user)
    await user.click(within(brief).getByRole('checkbox', { name: '목표 글자 수 사용' }))
    await user.clear(within(brief).getByLabelText('목표 글자 수'))
    await user.type(within(brief).getByLabelText('목표 글자 수'), '1500')
    await user.click(await m1())
    await user.click(within(brief).getByRole('button', { name: '카페' }))
    await user.click(within(brief).getByRole('checkbox', { name: '기억 사용' }))
    // Every change so far is the form's: nothing has been sent.
    expect(calls).not.toContain('SavePostGenerationOptions')

    await user.click(within(brief).getByRole('button', { name: '저장' }))
    await briefClosed()
    expect(optionSaves).toEqual([
      {
        slug: SLUG,
        targetLength: 1500,
        tagCount: 4,
        useMemory: true,
        qualityRules: ['title_saturation'],
        field: 'cafe',
      },
    ])

    const shows = async () => {
      const reopened = await openBrief(user)
      expect(within(reopened).getByLabelText('목표 글자 수')).toHaveValue(1500)
      expect(await m1()).toBeChecked()
      expect(within(reopened).getByRole('button', { name: '카페' })).toHaveAttribute(
        'aria-pressed',
        'true',
      )
      expect(within(reopened).getByRole('checkbox', { name: '기억 사용' })).toBeChecked()
    }
    await shows()

    first.unmount()
    renderAppAt(`/posts/${SLUG}`, { transport: first.transport })
    await shows()
    expect(optionSaves).toHaveLength(1)
  })

  it('offers no run options before the first save, and the create carries no 분야', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const draftSaves: FakeDraftSave[] = []
    renderAppAt('/posts/new', {
      user: USER,
      posts: { draftSaves },
      quality: { calls, accounts: { [SLUG]: OVER } },
    })

    expect(await screen.findByLabelText('제목')).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: '분야' })).toBeNull()
    expect(screen.queryByRole('checkbox', { name: '기억 사용' })).toBeNull()

    const brief = await openBrief(user)
    expect(await within(brief).findByRole('combobox', { name: /작성 모델/ })).toBeInTheDocument()
    expect(within(brief).queryByRole('checkbox', { name: '목표 글자 수 사용' })).toBeNull()
    expect(within(brief).queryByLabelText('태그 개수')).toBeNull()
    expect(within(brief).queryByText('발행 글 점검')).toBeNull()
    expect(within(brief).queryByRole('group', { name: '분야' })).toBeNull()
    expect(within(brief).queryByRole('checkbox', { name: '기억 사용' })).toBeNull()
    expect(calls.filter((call) => call === 'GetAccountQuality')).toHaveLength(0)
    await user.keyboard('{Escape}')

    await user.type(screen.getByLabelText('제목'), '리뷰 글')
    await waitFor(() => expect(draftSaves).toHaveLength(1), { timeout: 4_000 })
    expect(draftSaves[0]).toMatchObject({ slug: '', field: undefined })
  })
})

// QUAL-36, POST-83: ② carries this post's own M2, M3 and M4 above the article it measures, read at
// the revision on screen.
describe('the post measurement row', () => {
  const HEADING = '이 글의 측정값'
  const REVIEW = {
    slug: '20260820-measured',
    status: 'review',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
  }
  const MEASURED = {
    [REVIEW.slug]: [
      {
        metric: 'cross_post_phrases' as const,
        verdict: 'within_band' as const,
        minimum: 3,
        publishedCount: 4,
        values: { share: 0.05, shareWarnAbove: 0.1 },
      },
      {
        metric: 'in_post_repetition' as const,
        verdict: 'over_band' as const,
        values: {
          repetitionShare: 0.12,
          titleRelevance: 0.6,
          repetitionShareWarnAbove: 0.08,
          titleRelevanceWarnBelow: 0.5,
        },
      },
      {
        metric: 'composition' as const,
        verdict: 'within_band' as const,
        values: {
          charCount: 820,
          photoCount: 2,
          distinctBlockTypes: 3,
          averageSentenceLength: 12.5,
          distinctBlockTypesWarnAtOrBelow: 2,
        },
      },
    ],
  }
  const measurementReads = (calls: string[]) =>
    calls.filter((call) => call === 'GetPostMeasurement').length

  it('sits above the article on ②', async () => {
    // English content under the Korean interface: a sentence is counted in the content's words.
    renderAppAt(`/posts/${REVIEW.slug}`, {
      user: USER,
      posts: { posts: [{ ...REVIEW, contentLanguage: 'en' }] },
      quality: { measurements: MEASURED },
    })

    const region = await screen.findByRole('region', { name: HEADING })
    // Directly above the article, under 글 다듬기 and its save status.
    expect(region.nextElementSibling).toBe(screen.getByRole('article', { name: '생성된 글' }))
    expect(screen.getByRole('region', { name: '글 다듬기' })).toContainElement(region)
    expect(await within(region).findByText('본문 명사 중 가장 잦은 명사')).toBeInTheDocument()
    expect(within(region).getByText('주의')).toBeInTheDocument()
    expect(within(region).getAllByText('양호')).toHaveLength(2)
    expect(within(region).getByText('12.5단어')).toBeInTheDocument()
  })

  it('refetches after a block save', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const { queryClient } = renderAppAt(`/posts/${REVIEW.slug}`, {
      user: USER,
      calls,
      posts: { calls, posts: [REVIEW] },
      quality: { measurements: MEASURED },
    })
    const region = await screen.findByRole('region', { name: HEADING })
    await within(region).findByText('글자 수')
    expect(measurementReads(calls)).toBe(1)

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '측정을 다시 부르는 문단')
    await user.click(screen.getByRole('button', { name: '저장' }))

    // The save moves the revision the row reads, so the next revision is read — once the save
    // has landed, and not before.
    await waitFor(() => expect(calls).toContain('SavePostContent'), { timeout: 4_000 })
    await waitFor(() => expect(measurementReads(calls)).toBeGreaterThan(1))
    expect(calls.indexOf('SavePostContent')).toBeLessThan(calls.lastIndexOf('GetPostMeasurement'))
    // Read at the revision the save produced, which is what a generation moves too (QUAL-3).
    const revisions = queryClient
      .getQueryCache()
      .findAll({ queryKey: ['quality'] })
      .map((query) => query.queryKey.at(-1))
    expect(revisions).toContain('2')
    // The previous reading stays on screen while the next one loads: never the loading line again.
    expect(within(region).queryByText('측정하는 중이에요.')).toBeNull()
  })

  it('is hidden on a published post', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${REVIEW.slug}`, {
      user: USER,
      calls,
      posts: {
        calls,
        posts: [
          {
            ...REVIEW,
            status: 'published',
            finalizedRevision: 1n,
            finalizedAt: '2026-08-20T12:00:00Z',
            publishedUrl: 'https://blog.naver.com/alice/1',
            publishedAt: '2026-08-21T09:00:00Z',
          },
        ],
      },
      quality: { measurements: MEASURED },
    })

    await openStep(user, '글 다듬기')
    expect(await screen.findByRole('article', { name: '생성된 글' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: HEADING })).toBeNull()
    expect(measurementReads(calls)).toBe(0)
  })

  it('is absent while ② has no draft', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${REVIEW.slug}`, {
      user: USER,
      calls,
      posts: { calls, posts: [{ slug: REVIEW.slug, title: '제주' }] },
      quality: { measurements: MEASURED },
    })

    await openStep(user, '글 다듬기')
    expect(await screen.findByText(/아직 다듬을 글이 없어요/)).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: HEADING })).toBeNull()
    expect(measurementReads(calls)).toBe(0)
  })
})

// POST-81: ①'s brief reads the account aggregate and offers a tick on an over-band metric; the
// ticks autosave per post, where the next run's enqueue reads them.
describe('the brief quality rows', () => {
  const SLUG = '20260901-seongsu'
  const m1 = () => screen.findByRole('checkbox', { name: /^제목 도배율 42%/ })
  const reads = (calls: string[]) => calls.filter((call) => call === 'GetAccountQuality').length

  it('saves a tick and keeps it across a reload', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const optionSaves: FakeOptionsSave[] = []
    const first = renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: {
        calls,
        optionSaves,
        posts: [{ slug: SLUG, title: '성수 카페', targetLength: 1800 }],
      },
      quality: { accounts: { [SLUG]: OVER } },
    })

    // Read as soon as the dock renders, before anyone opens the brief.
    await screen.findByRole('tab', { name: '글 생성' })
    await waitFor(() => expect(reads(calls)).toBe(1))
    const brief = await openBrief(user)
    const box = await m1()
    expect(box).not.toBeChecked()
    await user.click(box)
    await user.click(within(brief).getByRole('button', { name: '저장' }))
    await waitFor(() =>
      expect(optionSaves).toEqual([
        {
          slug: SLUG,
          targetLength: 1800,
          tagCount: 4,
          useMemory: false,
          qualityRules: ['title_saturation'],
          field: '',
        },
      ]),
    )
    // The start requests carry nothing new: the enqueue reads the saved ticks (T344).
    expect(calls).not.toContain('StartGeneration')

    first.unmount()
    renderAppAt(`/posts/${SLUG}`, { transport: first.transport })
    await openBrief(user)
    expect(await m1()).toBeChecked()
  })

  it('re-reads the aggregate when the target language changes', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const { queryClient } = renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: { calls, posts: [{ slug: SLUG, title: '성수 카페' }] },
      quality: { accounts: { [SLUG]: OVER } },
    })

    const language = await briefField(user, '글 언어')
    await m1()
    const before = reads(calls)
    await user.click(language)
    await user.click(await screen.findByRole('option', { name: '영어' }))

    await waitFor(() => expect(calls).toContain('SavePostDraft'))
    await waitFor(() => expect(reads(calls)).toBeGreaterThan(before))
    // The rows on screen read the aggregate in the new language, whose rule texts are in it.
    await waitFor(() => {
      const shown = queryClient
        .getQueryCache()
        .findAll({ queryKey: ['quality'] })
        .filter((query) => query.queryKey[3] === 'account' && query.getObserversCount() > 0)
        .map((query) => query.queryKey[5])
      expect(shown).toEqual(['en'])
    })
  })

  it('refetches the aggregate after a URL save', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const { queryClient } = renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: {
        calls,
        posts: [
          {
            slug: SLUG,
            title: '성수 카페',
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
      quality: { accounts: { [SLUG]: OVER } },
    })

    await openBrief(user)
    await m1()
    await user.keyboard('{Escape}')
    await openStep(user, '글 완성')
    await user.click(await screen.findByLabelText('네이버 블로그 글 주소'))
    await user.paste('https://blog.naver.com/alice/223000000001')
    await user.click(
      within(screen.getByRole('region', { name: '발행' })).getByRole('button', { name: '저장' }),
    )
    await waitFor(() => expect(calls).toContain('SavePostPublishedUrl'))

    // The paste changed which posts are 발행됨, so the aggregate this brief read is stale (QUAL-3).
    await waitFor(() => {
      const account = queryClient
        .getQueryCache()
        .findAll({ queryKey: ['quality'] })
        .find((query) => query.queryKey[3] === 'account')
      expect(account?.state.isInvalidated).toBe(true)
    })
    const before = reads(calls)
    await openBrief(user)
    await waitFor(() => expect(reads(calls)).toBeGreaterThan(before))
  })

  it('disables the ticks while a job runs', async () => {
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [
          {
            slug: SLUG,
            title: '성수 카페',
            activeJob: { id: 'job-1', kind: 'generate', status: 'running' },
          },
        ],
      },
      jobs: { jobs: [{ id: 'job-1', kind: 'generate', status: 'running' }] },
      quality: { accounts: { [SLUG]: OVER } },
    })

    await openBrief(user)
    expect(await m1()).toBeDisabled()
    // The tip still explains the row while the box holds still.
    expect(screen.getByRole('button', { name: '제목 도배율 설명' })).toBeEnabled()
  })

  it('disables the ticks on a published post', async () => {
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [
          {
            slug: SLUG,
            title: '성수 카페',
            status: 'published',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
            finalizedRevision: 1n,
            finalizedAt: '2026-08-20T12:00:00Z',
            publishedUrl: 'https://blog.naver.com/alice/1',
            publishedAt: '2026-08-21T09:00:00Z',
            qualityRules: ['title_saturation'],
          },
        ],
      },
      quality: { accounts: { [SLUG]: OVER } },
    })

    await openStep(user, '글 생성')
    await openBrief(user)
    const box = await m1()
    expect(box).toBeDisabled()
    expect(box).toBeChecked()
    expect(screen.getByRole('button', { name: '제목 도배율 설명' })).toBeEnabled()
  })
})

// POST-79, POST-80: ② marks where the last write's candidates still stand, and taking a phrase is
// an ordinary content save that records nothing about where the words came from.
describe('the replacement marks', () => {
  const SLUG = '20260820-rain'
  const CANDIDATES: NonNullable<FakePostRow['replacementCandidates']> = [
    { surface: 'title', index: 0, source: '제주', phrases: ['제주도', '제주 바다'] },
    // '여행' would duplicate tag 2, so only '산책로' is offered.
    { surface: 'tag', index: 1, source: '산책', phrases: ['산책로', '여행'] },
    {
      surface: 'body',
      index: 0,
      source: '기다렸다',
      phrases: ['기다린다', '기다려 본다', '기다리고 있었다', '기다렸어요'],
    },
    { surface: 'body', index: 0, source: '비가', phrases: ['빗줄기가'] },
    { surface: 'body', index: 2, source: '바닷가로', phrases: ['해변으로'] },
  ]
  const finalized: FakePostRow = {
    slug: SLUG,
    status: 'finalized',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
    finalizedRevision: 1n,
    finalizedAt: '2026-08-20T12:00:00Z',
    replacementCandidates: CANDIDATES,
  }
  const article = () => within(screen.getByRole('article', { name: '생성된 글' }))

  function renderMarks(row: FakePostRow = finalized) {
    const calls: string[] = []
    const contentSaves: NonNullable<FakePostsOptions['contentSaves']> = []
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: { calls, contentSaves, posts: [row] },
    })
    return { calls, contentSaves }
  }

  async function openRefine(user: ReturnType<typeof userEvent.setup>) {
    await openStep(user, '글 다듬기')
    await screen.findByRole('article', { name: '생성된 글' })
  }

  it('marks the title, a tag and a TEXT block', async () => {
    const user = userEvent.setup()
    renderMarks()
    await openRefine(user)

    const heading = article().getByRole('heading', { level: 3 })
    const title = within(heading)
    expect(title.getByRole('button', { name: '제주' })).toHaveAttribute('aria-haspopup', 'dialog')
    // The text around a mark is kept whole.
    expect(heading).toHaveTextContent(/^비 온 뒤의 제주$/)
    expect(article().getByRole('button', { name: '비가' }).parentElement).toHaveTextContent(
      /^비가 그치기를 기다렸다\.$/,
    )
    const tags = within(article().getByRole('list', { name: '태그' }))
    // A chip stands alone, so its mark grows to the pointer floor under a coarse pointer; a mark in
    // a sentence takes WCAG 2.5.8's inline exception (THEME-23).
    expect(tags.getByRole('button', { name: '산책' })).toHaveClass(
      'pointer-coarse:min-h-11',
      'pointer-coarse:min-w-11',
    )
    expect(article().getByRole('button', { name: '기다렸다' })).not.toHaveClass(
      'pointer-coarse:min-h-11',
    )
    // The tag chip with no candidate stays plain text.
    expect(tags.queryByRole('button', { name: '제주' })).toBeNull()
    expect(article().getByRole('button', { name: '기다렸다' })).toBeInTheDocument()
    expect(article().getByRole('button', { name: '비가' })).toBeInTheDocument()
  })

  it('offers at most three phrases and no duplicate tag', async () => {
    const user = userEvent.setup()
    renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '기다렸다' }))
    const panel = within(await screen.findByRole('dialog', { name: '‘기다렸다’ 바꿔 쓰기' }))
    expect(panel.getAllByRole('button').map((button) => button.textContent)).toEqual([
      '기다린다',
      '기다려 본다',
      '기다리고 있었다',
    ])
    expect(panel.getByRole('button', { name: '‘기다린다’(으)로 바꾸기' })).toBeInTheDocument()
    expect(panel.getByText('‘기다렸다’ 대신 쓸 수 있는 표현')).toBeInTheDocument()
    // What was observed about the phrases, and nothing about what they gain.
    expect(
      panel.getByText('네이버 검색 결과의 제목과 설명에 자주 나온 표현이에요.'),
    ).toBeInTheDocument()
    await user.keyboard('{Escape}')

    await user.click(article().getByRole('button', { name: '산책' }))
    const tagPanel = within(await screen.findByRole('dialog', { name: '‘산책’ 바꿔 쓰기' }))
    expect(tagPanel.getAllByRole('button').map((button) => button.textContent)).toEqual(['산책로'])
  })

  it('saves a take with the expected revision and returns 확정 to 검토', async () => {
    const user = userEvent.setup()
    const { contentSaves } = renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '기다렸다' }))
    await user.click(await screen.findByRole('button', { name: '‘기다려 본다’(으)로 바꾸기' }))

    await waitFor(() => expect(contentSaves).toHaveLength(1), { timeout: 4_000 })
    expect(contentSaves[0].slug).toBe(SLUG)
    expect(contentSaves[0].expectedRevision).toBe(1n)
    expect(contentSaves[0].content.blocks[0].content).toBe('비가 그치기를 기다려 본다.')
    // The rest of the content went as the editor held it.
    expect(contentSaves[0].content.title).toBe(POST_CONTENT_FIXTURE.title)
    expect(contentSaves[0].content.tags).toEqual(POST_CONTENT_FIXTURE.tags)
    // Focus lands on the pencil of the block that held the text, never on <body>.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '1번째 블록 수정' })).toHaveFocus(),
    )
    await waitFor(() =>
      expect(screen.getByRole('status', { name: '글 상태' })).toHaveTextContent('검토'),
    )
    expect(screen.getByRole('tab', { name: '글 다듬기' })).toHaveAttribute('aria-selected', 'true')
  })

  // A phrase can contain its own source, which then still stands: the mark stays, and the panel
  // the take came from closes all the same. Focus goes to the header's pencil.
  it('closes the panel on a take, even where the source still stands', async () => {
    const user = userEvent.setup()
    const { contentSaves } = renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '제주' }))
    await user.click(await screen.findByRole('button', { name: '‘제주도’(으)로 바꾸기' }))

    expect(screen.queryByRole('dialog', { name: '‘제주’ 바꿔 쓰기' })).toBeNull()
    await waitFor(() => expect(contentSaves).toHaveLength(1), { timeout: 4_000 })
    expect(contentSaves[0].content.title).toBe('비 온 뒤의 제주도')
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '제목과 요약, 태그 수정' })).toHaveFocus(),
    )
    expect(article().getByRole('heading', { level: 3 })).toHaveTextContent(/^비 온 뒤의 제주도$/)
  })

  it('puts focus on the pencil of the block the take changed', async () => {
    const user = userEvent.setup()
    const { contentSaves } = renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '바닷가로' }))
    await user.click(await screen.findByRole('button', { name: '‘해변으로’(으)로 바꾸기' }))

    await waitFor(() =>
      expect(screen.getByRole('button', { name: '3번째 블록 수정' })).toHaveFocus(),
    )
    await waitFor(() => expect(contentSaves).toHaveLength(1), { timeout: 4_000 })
    expect(contentSaves[0].content.blocks[2].content).toBe('해변으로')
  })

  it('keeps a neighbouring mark after a take', async () => {
    const user = userEvent.setup()
    renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '기다렸다' }))
    await user.click(await screen.findByRole('button', { name: '‘기다린다’(으)로 바꾸기' }))

    await waitFor(() => expect(article().queryByRole('button', { name: '기다렸다' })).toBeNull())
    expect(article().getByText(/기다린다\./)).toBeInTheDocument()
    expect(article().getByRole('button', { name: '비가' })).toBeInTheDocument()
    expect(article().getByRole('button', { name: '제주' })).toBeInTheDocument()
  })

  it('drops a mark whose source was edited away', async () => {
    const user = userEvent.setup()
    renderMarks()
    await openRefine(user)

    await user.click(screen.getByRole('button', { name: '1번째 블록 수정' }))
    // An open editor is plain fields: no mark stands in the block it edits.
    expect(article().queryByRole('button', { name: '비가' })).toBeNull()
    const field = screen.getByLabelText('1번째 블록 내용')
    await user.clear(field)
    await user.type(field, '비가 그쳤다.')
    await user.click(screen.getByRole('button', { name: '저장' }))

    expect(article().queryByRole('button', { name: '기다렸다' })).toBeNull()
    expect(article().getByRole('button', { name: '비가' })).toBeInTheDocument()
  })

  it('sends nothing for an ignored mark', async () => {
    const user = userEvent.setup()
    const { calls } = renderMarks()
    await openRefine(user)

    await user.click(article().getByRole('button', { name: '제주' }))
    expect(await screen.findByRole('dialog', { name: '‘제주’ 바꿔 쓰기' })).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog', { name: '‘제주’ 바꿔 쓰기' })).toBeNull()

    // Past the content debounce: opening and closing a mark is no edit (POST-79).
    await new Promise((resolve) => setTimeout(resolve, 1_500))
    expect(calls).not.toContain('SavePostContent')
  })

  it('shows no marks on a published post', async () => {
    const user = userEvent.setup()
    renderMarks({
      ...finalized,
      status: 'published',
      publishedUrl: 'https://blog.naver.com/alice/1',
      publishedAt: '2026-08-21T09:00:00Z',
    })
    await openRefine(user)

    expect(article().getByText('비가 그치기를 기다렸다.')).toBeInTheDocument()
    expect(article().queryByRole('button')).toBeNull()
  })
})

// POST-86: a published post takes no write but its address and the delete. It opens on 글 완성,
// ② reads its prose, ① shows its material read-only and says why exactly once, and a save refused
// because the post was published elsewhere is an answer rather than an outage.
describe('a published post', () => {
  const LOCKED = '발행된 글은 바꿀 수 없어요. 글 완성에서 발행 URL을 지우면 다시 고칠 수 있어요.'
  const WRITES = [
    'SavePostDraft',
    'SavePostContent',
    'SavePostGenerationOptions',
    'CreateUpload',
    'DeleteImage',
    'DeleteVideo',
    'FinalizePost',
    'StartGeneration',
    'StartWriteExperiment',
  ]
  const WITH_FIELDS = [
    {
      id: 'template-review',
      name: '정보성 식당 리뷰',
      body: '<write>인트로</write>\n<ask label="방문일"/>\n<ask label="총평 별점">별점과 총평</ask>',
    },
  ]
  const published = {
    slug: '20260820-seongsu',
    title: '성수 카페 투어',
    memo: '라떼가 맛있었다',
    status: 'published',
    content: POST_CONTENT_FIXTURE,
    images: POST_IMAGES_FIXTURE,
    videos: [{ id: 'video-1', filename: 'CLIP_1.mp4' }],
    template: { id: 'template-review', name: '정보성 식당 리뷰' },
    templateAnswers: [{ label: '방문일', text: '9월 20일', enabled: true }],
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
    finalizedRevision: 1n,
    finalizedAt: '2026-08-20T12:00:00Z',
    publishedUrl: 'https://blog.naver.com/alice/1',
    publishedAt: '2026-08-21T09:00:00Z',
  }
  // Every model the post would need is chosen, so the lock is the only reason ① could give.
  const PROVIDERS = {
    models: [
      {
        providerId: 'openrouter',
        modelId: 'seer',
        vision: true,
        videoInput: true,
        signedVideoUrl: true,
      },
      { providerId: 'openrouter', modelId: 'writer' },
      { providerId: 'openrouter', modelId: 'writer-b' },
      { providerId: 'openrouter', modelId: 'analyzer' },
    ],
    selections: [
      { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'seer' },
      { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
      { stage: Stage.ANALYZE, providerId: 'openrouter', modelId: 'analyzer' },
    ],
    comparisonPairs: [
      {
        stage: Stage.WRITE,
        candidateA: { providerId: 'openrouter', modelId: 'writer' },
        candidateB: { providerId: 'openrouter', modelId: 'writer-b' },
      },
    ],
  }
  const statusLine = () => screen.getByRole('status', { name: '글 상태' })
  const selected = (name: string) =>
    expect(screen.getByRole('tab', { name })).toHaveAttribute('aria-selected', 'true')

  function renderPublished(calls: string[] = []) {
    return renderAppAt(`/posts/${published.slug}`, {
      user: USER,
      calls,
      posts: {
        calls,
        posts: [published],
        templates: [{ id: 'template-review', name: '정보성 식당 리뷰' }],
      },
      templates: { templates: WITH_FIELDS },
      providers: PROVIDERS,
    })
  }

  it('lands on 글 완성 with learning and export available, and the line reads 발행됨', async () => {
    renderPublished()

    const learn = await screen.findByRole('button', { name: '말투 학습' })
    selected('글 완성')
    // The same gates as a finalized post: publishing keeps the finalized revision (POST-21).
    await waitFor(() => expect(learn).toBeEnabled())
    expect(screen.getByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    await waitFor(() => expect(statusLine()).toHaveTextContent('발행됨'))
  })

  it('keeps 문장 의견 on 글 완성 after a completed learning run', async () => {
    const key = `postpilot:voice-learning:alice:${published.slug}`
    const stored = new Map<string, string>()
    vi.stubGlobal('localStorage', {
      getItem: (name: string) => stored.get(name) ?? null,
      setItem: (name: string, value: string) => stored.set(name, value),
      removeItem: (name: string) => stored.delete(name),
    })
    localStorage.setItem(
      key,
      JSON.stringify({ eventId: 'event-1', jobId: 'learn-1', contentRevision: '1' }),
    )
    renderAppAt(`/posts/${published.slug}`, {
      user: USER,
      posts: { posts: [published] },
      jobs: { jobs: [{ id: 'learn-1', kind: 'voice_learn', status: 'done' }] },
      providers: PROVIDERS,
    })

    expect(await screen.findByText('이 글에서 말투를 배웠어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '문장 의견' })).toBeInTheDocument()
  })

  it('reads ② as prose under the one sentence, with only the road onward in its dock', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderPublished(calls)
    await screen.findByRole('button', { name: '말투 학습' })

    await openStep(user, '글 다듬기')
    const notice = await screen.findByText(LOCKED)
    expect(notice.closest('[role="status"]')).not.toBeNull()
    const prose = screen.getByRole('article', { name: '생성된 글' })
    expect(within(prose).getByRole('heading', { name: '비 온 뒤의 제주' })).toBeInTheDocument()
    expect(within(prose).getByText('비가 그치기를 기다렸다.')).toBeInTheDocument()
    // No block editor: nothing edits or moves a block, and nothing can start a content save.
    expect(screen.queryByRole('button', { name: '제목과 요약, 태그 수정' })).toBeNull()
    expect(screen.queryByRole('button', { name: '문단 추가' })).toBeNull()
    expect(screen.queryByRole('button', { name: /번째 블록/ })).toBeNull()

    const dock = screen.getByLabelText('글 작업')
    const onward = within(dock).getByRole('button', { name: '글 완성으로 가기' })
    expect(within(dock).getAllByRole('button')).toEqual([onward])
    expect(screen.queryByLabelText('수정 요청을 입력하세요')).toBeNull()

    await user.click(onward)
    await waitFor(() => selected('글 완성'))
    expect(calls.filter((call) => WRITES.includes(call))).toEqual([])
  })

  it('shows ① read-only, says why once and sends no write', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderPublished(calls)
    await screen.findByRole('button', { name: '말투 학습' })

    await openStep(user, '글 생성')
    const title = await screen.findByLabelText('제목')
    expect(title).toHaveAttribute('readonly')
    expect(title).toHaveValue('성수 카페 투어')
    const memo = screen.getByLabelText('메모')
    expect(memo).toHaveAttribute('readonly')
    expect(memo).toHaveValue('라떼가 맛있었다')
    // Typing into a read-only field changes nothing, so nothing is queued.
    await user.type(title, ' 후기')
    await user.type(memo, ' 또')
    expect(title).toHaveValue('성수 카페 투어')

    expect(await screen.findByLabelText('방문일')).toBeDisabled()
    expect(screen.getByLabelText('방문일')).toHaveValue('9월 20일')
    expect(screen.getByRole('switch', { name: '총평 별점 넣기' })).toBeDisabled()
    // The picker's two file inputs, each labelled by its button-styled label.
    expect(screen.getByLabelText('사진·영상 추가')).toBeDisabled()
    expect(screen.getByLabelText('촬영')).toBeDisabled()
    expect(screen.getByRole('img', { name: 'IMG_1.jpg' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /삭제$/ })).toBeNull()
    expect(await voiceField(user)).toBeDisabled()
    expect(await templateField(user)).toBeDisabled()

    const dock = screen.getByLabelText('글 작업')
    await waitFor(() => expect(within(dock).getByRole('button', { name: '생성' })).toBeDisabled())
    expect(within(dock).getByRole('button', { name: 'A/B 비교' })).toBeDisabled()
    // ONE sentence on the whole screen, and no other reason beside any control of ①.
    expect(screen.getAllByText(LOCKED)).toHaveLength(1)
    expect(
      within(dock)
        .getAllByRole('status')
        .map((status) => status.textContent),
    ).toEqual([LOCKED])
    expect(screen.queryByText(/^(생성|A\/B 비교):/)).toBeNull()

    // The brief: the post's own 글 언어 and options are shown and not changed, while the model
    // selects are the account's settings and stay usable.
    const brief = await openBrief(user)
    expect(within(brief).getByRole('combobox', { name: /^관찰 모델/ })).toBeEnabled()
    expect(within(brief).getByRole('combobox', { name: /^작성 모델/ })).toBeEnabled()
    expect(within(brief).getByRole('combobox', { name: /^글 언어/ })).toBeDisabled()
    expect(within(brief).getByRole('checkbox', { name: '목표 글자 수 사용' })).toBeDisabled()
    expect(within(brief).getByLabelText('태그 개수')).toBeDisabled()
    expect(within(brief).getByRole('checkbox', { name: '기억 사용' })).toBeDisabled()
    for (const chip of within(within(brief).getByRole('group', { name: '분야' })).getAllByRole(
      'button',
    ))
      expect(chip).toBeDisabled()
    expect(within(brief).getByRole('button', { name: '저장' })).toBeDisabled()

    // Past the debounce: still nothing sent.
    await new Promise((resolve) => setTimeout(resolve, 1_500))
    expect(calls.filter((call) => WRITES.includes(call))).toEqual([])
  })

  it('takes a draft save refused as published for an answer, and ① locks after the refetch', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const slug = '20260820-jeju'
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      calls,
      posts: {
        calls,
        posts: [
          {
            slug,
            title: '제주',
            memo: '갔다',
            status: 'finalized',
            content: POST_CONTENT_FIXTURE,
            contentRevision: 1n,
            machineBaselineRevision: 1n,
            canFinalize: true,
            finalizedRevision: 1n,
          },
        ],
        // Published from another tab between two autosaves.
        publishOnDraftSave: slug,
      },
    })

    await openStep(user, '글 생성')
    await user.type(await screen.findByLabelText('제목'), ' 여행기')
    await waitFor(() => expect(calls).toContain('SavePostDraft'), { timeout: 4_000 })

    // The refusal refetches the post, which now reads published and carries the reader to ③.
    await waitFor(() => expect(statusLine()).toHaveTextContent('발행됨'), { timeout: 4_000 })
    await waitFor(() => selected('글 완성'))
    expect(screen.queryByText('저장하지 못했어요 · 다시 시도 중')).toBeNull()

    // Past two retry windows: the save went out once and was never retried.
    await new Promise((resolve) => setTimeout(resolve, 3_000))
    expect(calls.filter((call) => call === 'SavePostDraft')).toHaveLength(1)
    expect(screen.queryByText('저장하지 못했어요 · 다시 시도 중')).toBeNull()

    await openStep(user, '글 생성')
    const title = await screen.findByLabelText('제목')
    expect(title).toHaveAttribute('readonly')
    // The server's, not the text the lock refused.
    expect(title).toHaveValue('제주')
    expect(screen.getAllByText(LOCKED)).toHaveLength(1)
  }, 15_000)

  // Reopening is clearing the post's URL on 글 완성 (`published → finalized`). The fields start
  // again from the server's, not from the text the lock refused.
  it('reseeds ① from the server when the post reopens', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const slug = '20260820-jeju'
    const finalized = {
      slug,
      title: '제주',
      memo: '갔다',
      status: 'finalized',
      content: POST_CONTENT_FIXTURE,
      contentRevision: 1n,
      machineBaselineRevision: 1n,
      canFinalize: true,
      finalizedRevision: 1n,
    }
    const { queryClient } = renderAppAt(`/posts/${slug}`, {
      user: USER,
      calls,
      posts: {
        calls,
        posts: [finalized],
        getSequence: [
          finalized,
          { ...finalized, status: 'published', publishedUrl: 'https://blog.naver.com/alice/1' },
          finalized,
        ],
        publishOnDraftSave: slug,
      },
    })

    await openStep(user, '글 생성')
    await user.type(await screen.findByLabelText('제목'), ' 여행기')
    await user.type(screen.getByLabelText('메모'), ' 또')
    await waitFor(() => expect(statusLine()).toHaveTextContent('발행됨'), { timeout: 4_000 })

    // The URL was cleared elsewhere: the next read finds the post finalized again.
    await queryClient.invalidateQueries()
    await waitFor(() => expect(statusLine()).toHaveTextContent('확정'))
    await openStep(user, '글 생성')
    const title = await screen.findByLabelText('제목')
    expect(title).not.toHaveAttribute('readonly')
    expect(title).toHaveValue('제주')
    expect(screen.getByLabelText('메모')).toHaveValue('갔다')
    expect(screen.queryByText(LOCKED)).toBeNull()
  }, 15_000)
})

// POST-73, POST-75, POST-77, POST-87: ③'s foot records, replaces and clears the post's Naver
// address, refuses anything else before sending, and waits for 확정.
describe('the 발행 URL field', () => {
  const slug = '20260820-seongsu'
  const finalizedPost = {
    slug,
    title: '성수 카페',
    status: 'finalized',
    content: POST_CONTENT_FIXTURE,
    contentRevision: 1n,
    machineBaselineRevision: 1n,
    canFinalize: true,
    finalizedRevision: 1n,
    finalizedAt: '2026-08-20T12:00:00Z',
  }
  const publishedPost = {
    ...finalizedPost,
    status: 'published',
    publishedUrl: 'https://blog.naver.com/alice/1',
    publishedAt: '2026-08-21T09:00:00Z',
  }
  const statusLine = () => screen.getByRole('status', { name: '글 상태' })
  const section = () => screen.getByRole('region', { name: '발행' })
  const field = () => within(section()).getByLabelText('네이버 블로그 글 주소')
  const saveButton = () => within(section()).getByRole('button', { name: '저장' })

  function renderPost(row: FakePostRow, extra: FakePostsOptions = {}) {
    const calls: string[] = []
    const saves: string[] = []
    renderAppAt(`/posts/${slug}`, {
      user: USER,
      calls,
      posts: { calls, posts: [row], publishedUrlSaves: saves, ...extra },
    })
    return { calls, saves }
  }

  it('is a url field at the foot of ③, typed for the keyboard that pastes an address', async () => {
    renderPost(finalizedPost)
    const input = await screen.findByLabelText('네이버 블로그 글 주소')
    for (const [name, value] of Object.entries({
      type: 'url',
      inputmode: 'url',
      autocomplete: 'url',
      autocapitalize: 'none',
      autocorrect: 'off',
      enterkeyhint: 'done',
    })) {
      expect(input).toHaveAttribute(name, value)
    }
    expect(input).toHaveValue('')
    expect(saveButton()).toBeEnabled()
    expect(within(section()).queryByRole('button', { name: '지우기' })).toBeNull()
  })

  it('publishes on a pasted address and stays on ③', async () => {
    const user = userEvent.setup()
    const { saves } = renderPost(finalizedPost)
    await user.click(await screen.findByLabelText('네이버 블로그 글 주소'))
    await user.paste('  https://blog.naver.com/alice/223000000001  ')
    await user.click(saveButton())

    await waitFor(() => expect(statusLine()).toHaveTextContent('발행됨'))
    expect(saves).toEqual(['https://blog.naver.com/alice/223000000001'])
    expect(screen.getByRole('tab', { name: '글 완성' })).toHaveAttribute('aria-selected', 'true')
    expect(field()).toHaveValue('https://blog.naver.com/alice/223000000001')
  })

  it('accepts the mobile share address and shows it normalized', async () => {
    const user = userEvent.setup()
    const { saves } = renderPost(finalizedPost)
    await user.type(
      await screen.findByLabelText('네이버 블로그 글 주소'),
      'https://m.blog.naver.com/alice/7{Enter}',
    )

    await waitFor(() => expect(field()).toHaveValue('https://blog.naver.com/alice/7'))
    // What is sent is the input as typed, trimmed; the normalization is the server's answer.
    expect(saves).toEqual(['https://m.blog.naver.com/alice/7'])
    expect(statusLine()).toHaveTextContent('발행됨')
  })

  it('refuses another address in place and sends nothing', async () => {
    const user = userEvent.setup()
    const { calls } = renderPost(finalizedPost)
    const input = await screen.findByLabelText('네이버 블로그 글 주소')
    for (const bad of [
      'https://cafe.naver.com/alice/1',
      'https://blog.naver.com:443/alice/1',
      'https://user@blog.naver.com/alice/1',
    ]) {
      await user.clear(input)
      await user.type(input, bad)
      await user.click(saveButton())
      const message = await within(section()).findByRole('alert')
      expect(message).toHaveTextContent('네이버 블로그 글 주소만 저장할 수 있어요.')
      expect(input).toHaveAttribute('aria-invalid', 'true')
      expect(input.getAttribute('aria-describedby')).toContain(message.id)
    }
    expect(calls).not.toContain('SavePostPublishedUrl')
    expect(statusLine()).toHaveTextContent('확정')
  })

  it('sends nothing for an empty 저장 on a post with no address', async () => {
    const user = userEvent.setup()
    const { calls } = renderPost(finalizedPost)
    await screen.findByLabelText('네이버 블로그 글 주소')
    await user.click(saveButton())
    expect(calls).not.toContain('SavePostPublishedUrl')
  })

  it('replaces a published address and stays 발행됨', async () => {
    const user = userEvent.setup()
    const { saves } = renderPost(publishedPost)
    const input = await screen.findByLabelText('네이버 블로그 글 주소')
    expect(input).toHaveValue('https://blog.naver.com/alice/1')
    await user.clear(input)
    await user.type(input, 'https://blog.naver.com/alice/2')
    await user.click(saveButton())

    await waitFor(() => expect(field()).toHaveValue('https://blog.naver.com/alice/2'))
    expect(saves).toEqual(['https://blog.naver.com/alice/2'])
    expect(statusLine()).toHaveTextContent('발행됨')
  })

  it.each([
    [
      '지우기',
      async (user: ReturnType<typeof userEvent.setup>) =>
        user.click(within(section()).getByRole('button', { name: '지우기' })),
    ],
    [
      'an emptied field and 저장',
      async (user: ReturnType<typeof userEvent.setup>) => {
        await user.clear(field())
        await user.click(saveButton())
      },
    ],
  ])('clears by %s back to 확정, and ② is editable again', async (_name, clear) => {
    const user = userEvent.setup()
    const { saves } = renderPost(publishedPost)
    await screen.findByLabelText('네이버 블로그 글 주소')
    await clear(user)

    await waitFor(() => expect(statusLine()).toHaveTextContent('확정'))
    expect(saves).toEqual([''])
    expect(field()).toHaveValue('')
    await openStep(user, '글 다듬기')
    expect(
      await screen.findByRole('button', { name: '제목과 요약, 태그 수정' }),
    ).toBeInTheDocument()
  })

  it('stays closed with its reason until the post is 확정', async () => {
    const user = userEvent.setup()
    const { calls } = renderPost({ ...finalizedPost, status: 'review', finalizedRevision: 0n })
    await openStep(user, '글 완성')
    const input = await screen.findByLabelText('네이버 블로그 글 주소')
    expect(input).toBeDisabled()
    expect(saveButton()).toBeDisabled()
    const reason = within(section()).getByText('글을 확정하면 발행 URL을 입력할 수 있어요.')
    expect(input.getAttribute('aria-describedby')).toContain(reason.id)
    expect(calls).not.toContain('SavePostPublishedUrl')
  })

  it('says a busy refusal in place and leaves the post as it was', async () => {
    const user = userEvent.setup()
    renderPost(finalizedPost, { publishedUrlBusy: true })
    await user.type(
      await screen.findByLabelText('네이버 블로그 글 주소'),
      'https://blog.naver.com/alice/1',
    )
    await user.click(saveButton())

    expect(await within(section()).findByRole('alert')).toHaveTextContent(
      '이 글에서 다른 작업이 진행 중이에요.',
    )
    expect(statusLine()).toHaveTextContent('확정')
    expect(field()).toHaveValue('https://blog.naver.com/alice/1')
  })
})
