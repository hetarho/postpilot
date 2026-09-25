// ① 글 생성: starting a run, the re-observation picker, the photos and the writing brief's run
// options and quality rows.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ExperimentOrigin, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import {
  BRIEF_TRIGGER,
  M1_OVER_BAND,
  USER,
  briefField,
  openBrief,
  resetEditorTest,
} from '@/test/editor'
import {
  OBSERVATION_FIXTURE,
  POST_CONTENT_FIXTURE,
  OBSERVATIONS_FIXTURE,
  POST_IMAGES_FIXTURE,
} from '@/test/fixtures/postContent'
import type { FakeWriteExperimentStart } from '@/test/experiments'
import type { FakeGenerationStart } from '@/test/jobs'
import { FAKE_STORAGE_ORIGIN, type FakeDraftSave, type FakeOptionsSave } from '@/test/posts'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

afterEach(() => {
  resetEditorTest()
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
})

describe('opening a post', () => {
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
})

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
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
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
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
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
      quality: { calls, accounts: { [SLUG]: M1_OVER_BAND } },
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
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
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
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: { calls, posts: [{ slug: SLUG, title: '성수 카페' }] },
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
    })

    const language = await briefField(user, '글 언어')
    await m1()
    const before = reads(calls)
    await user.click(language)
    await user.click(await screen.findByRole('option', { name: '영어' }))

    await waitFor(() => expect(calls).toContain('SavePostDraft'))
    await waitFor(() => expect(reads(calls)).toBeGreaterThan(before))
    // The server renders rule texts in the post's stored target language, and the read carries
    // only the slug, so a read after the language save is the rows reading the new language.
    expect(calls.lastIndexOf('GetAccountQuality')).toBeGreaterThan(calls.indexOf('SavePostDraft'))
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
      quality: { accounts: { [SLUG]: M1_OVER_BAND } },
    })

    await openBrief(user)
    expect(await m1()).toBeDisabled()
  })
})
