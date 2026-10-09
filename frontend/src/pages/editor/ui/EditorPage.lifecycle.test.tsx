// The editor page's frame: opening a post, a new draft, the delete, the three steps and the one
// status line.
import { afterEach, describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { USER, openBrief, openStep, resetEditorTest, stubBrowserImagePipeline } from '@/test/editor'
import { POST_CONTENT_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import { FAKE_STORAGE_ORIGIN, finalizedPostRow } from '@/test/posts'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

afterEach(() => {
  resetEditorTest()
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
})

describe('opening a post', () => {
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

  it('retains a failed paid comparison as a separate result follow-up', async () => {
    const user = userEvent.setup()
    const draftMaterials: Array<{ slug: string; title: string; memo: string }> = []
    const failed = {
      id: 'comparison-job',
      kind: 'model_experiment',
      status: 'failed',
      failureReason: 'MODEL_OUTPUT_INVALID' as const,
    }
    const { router } = renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: {
        draftMaterials,
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

    await screen.findByRole('tab', { name: '글 다듬기' })
    // Independent tests are not an ordinary post job; their retained result stays reachable.
    expect(screen.queryByText('AI 결과 형식을 읽을 수 없어요.')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()

    await openStep(user, '글 생성')
    fireEvent.change(screen.getByLabelText('메모'), { target: { value: '마지막 기록' } })
    const retained = await screen.findByRole('link', { name: 'A/B 결과 확인' })
    expect(retained).toHaveAttribute(
      'href',
      '/tests/records/experiment-pending?source=20260820-jeju',
    )
    await user.click(retained)
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/tests/records/experiment-pending'),
    )
    expect(router.state.location.search.source).toBe('20260820-jeju')
    expect(draftMaterials.at(-1)?.memo).toBe('마지막 기록')
  })

  // Someone else's slug is 403, not 404 (POST-3).
  it('reports a slug that belongs to someone else as theirs, not as missing', async () => {
    renderAppAt('/posts/20260101-hers', {
      user: USER,
      posts: { foreign: ['20260101-hers'] },
    })

    expect(await screen.findByRole('alert')).toHaveTextContent('다른 사람의 글이에요')
    expect(screen.getByRole('link', { name: '작업 내역으로 돌아가기' })).toBeInTheDocument()
  })

  it('reports an unknown slug as missing', async () => {
    renderAppAt('/posts/20260101-ghost', { user: USER })

    expect(await screen.findByRole('alert')).toHaveTextContent('없는 글이에요')
    // Only a failure that is not an answer is worth asking again.
    expect(screen.queryByRole('button', { name: '다시 시도' })).not.toBeInTheDocument()
  })
})

// Real timers on template: these walk the whole flow through the router, and
// @testing-library's async helpers look for jest's fake-timer API, so vitest's is
// invisible to them and every `waitFor` would spin on a clock nothing advances. The
// debounce window itself is covered by features/save-draft's own tests.
describe('the delete control', () => {
  // POST-27: an existing post can be deleted from its own editor.
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

  // POST-7, POST-34 through the editor: the first photo of a new draft creates the post,
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

  // POST-7: a pick with nothing to upload is reported and creates no post.
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

  // POST-44. The mapping is pages/editor/model/steps.test.ts and useDraftSteps.test.ts; one
  // row pins that the page follows it, and 'published' is "lands on 글 완성…".
  it.each([['review', '글 다듬기']])('opens a %s post on %s', async (status, label) => {
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, status, canFinalize: true }] },
    })

    const tab = await screen.findByRole('tab', { name: label })
    await waitFor(() => expect(tab).toHaveAttribute('aria-selected', 'true'))
  })

  // POST-54: each step renders its own panel and none of the others'.
  it('scopes the generation controls, the block surface, and finalize to their own steps', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, canFinalize: true }] },
    })

    // ② is where a review post opens: the draft, and the dock that carries the revision and the
    // one way out, 확정하기, which finalizes at once and opens nothing (POST-56). No 가제, no
    // writing brief, no generation control.
    expect(await screen.findByRole('heading', { name: '글 다듬기' })).toBeInTheDocument()
    expect(screen.queryByLabelText('제목')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /옵션/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '바로 글 쓰기' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '내보내기' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('수정 요청을 입력하세요')).toBeInTheDocument()
    const finalize = screen.getByRole('button', { name: '확정하기' })
    expect(finalize).not.toHaveAttribute('aria-haspopup')
    expect(screen.queryByRole('button', { name: '확정하고 말투 학습' })).not.toBeInTheDocument()

    await openStep(user, '글 생성')
    expect(await screen.findByLabelText('제목')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '글 다듬기' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('수정 요청을 입력하세요')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()
    const brief = await openBrief(user)
    expect(within(brief).queryByRole('combobox', { name: /후보 A/ })).toBeNull()
    await user.keyboard('{Escape}')

    // ③ carries 기억으로 저장, the manual export and the 발행 URL field, and nothing that would
    // teach a voice (POST-54).
    await openStep(user, '글 완성')
    expect(await screen.findByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '기억으로 저장' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '말투 학습' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '말투 학습' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('제목')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '바로 글 쓰기' })).not.toBeInTheDocument()
  })

  // POST-44: a step with no work yet says so and offers the way to the step that produces it.
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

  // POST-44: steps are panels, so a save started before a step change still completes.
  // The 가제 now belongs to 글 생성 alone (POST-50), so this also proves that unmounting the
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

  // POST-47, POST-57: ① and ② both always dock — 생성 ends the first step and
  // 확정 the second — while ③ still docks only when there is something to report. There is
  // exactly ONE bar in the scroller on every step (THEME-24).
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
    expect(within(generateDock).getByRole('button', { name: '바로 글 쓰기' })).toBeInTheDocument()
    expect(within(generateDock).getByRole('button', { name: /옵션/ })).toBeInTheDocument()
    expect(within(generateDock).queryByRole('button', { name: '확정하기' })).not.toBeInTheDocument()

    await openStep(user, '글 완성')
    expect(await screen.findByRole('button', { name: '기억으로 저장' })).toBeInTheDocument()
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
    const generate = await within(dock).findByRole('button', { name: '바로 글 쓰기' })
    await waitFor(() => expect(generate).toBeEnabled())
    expect(within(dock).getByRole('button', { name: '스토리라인 먼저' })).toBeEnabled()
    expect(within(dock).queryByRole('button', { name: '다른 방법으로 쓰기' })).toBeNull()
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

  // POST-54: the photos are the material the post is written from, so ① starts with them — on a
  // saved draft and on a /posts/new that has no post yet alike.
  it('puts the photos above the title, and the memo under it', async () => {
    const following = (first: Node, second: Node) =>
      Boolean(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING)
    const expectPhotosFirst = async () => {
      const picker = await screen.findByLabelText('사진·영상 추가')
      const title = screen.getByLabelText('제목')
      expect(following(picker, title)).toBe(true)
      expect(following(title, screen.getByLabelText('메모'))).toBe(true)
    }

    const saved = renderAppAt('/posts/20260820-jeju', {
      user: USER,
      posts: { posts: [{ ...reviewPost, status: 'draft' }] },
    })
    await expectPhotosFirst()
    saved.unmount()

    renderAppAt('/posts/new', { user: USER })
    await expectPhotosFirst()
  })
})

// POST-45: everything the editor has to SAY about its own state is one 2px bar plus one line at
// the top of the page, and the dock below it holds only controls and the reason one is refused.
describe('the editor status region', () => {
  const statusLine = () => screen.getByRole('status', { name: '글 상태' })

  it('reports the post status on the line when nothing else is happening', async () => {
    renderAppAt('/posts/20260820-final', {
      user: USER,
      posts: {
        posts: [finalizedPostRow({ slug: '20260820-final' })],
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
  it('retains the paid comparison link without blocking ordinary writing', async () => {
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
    await waitFor(() => expect(screen.getByRole('button', { name: '바로 글 쓰기' })).toBeEnabled())
    expect(screen.getByRole('button', { name: '스토리라인 먼저' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: '다른 방법으로 쓰기' })).toBeNull()
    expect(screen.queryByText('먼저 대기 중인 A/B 결과를 확인해 주세요.')).not.toBeInTheDocument()
    expect(screen.queryByText(/^생성:/)).not.toBeInTheDocument()
  })
})
