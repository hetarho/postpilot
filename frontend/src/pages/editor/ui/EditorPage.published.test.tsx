// POST-86: a published post takes no write but its address and the delete.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import {
  GUIDE,
  M1_OVER_BAND,
  USER,
  guideTo,
  openBrief,
  openStep,
  resetEditorTest,
  templateField,
  voiceField,
} from '@/test/editor'
import { OBSERVATION_FIXTURE, POST_IMAGES_FIXTURE } from '@/test/fixtures/postContent'
import { finalizedPostRow, publishedPostRow } from '@/test/posts'
import type { FakeQualityReading } from '@/test/quality'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

afterEach(() => {
  resetEditorTest()
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
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
  const published = publishedPostRow({
    slug: '20260820-seongsu',
    title: '성수 카페 투어',
    memo: '라떼가 맛있었다',
    images: POST_IMAGES_FIXTURE,
    videos: [{ id: 'video-1', filename: 'CLIP_1.mp4' }],
    template: { id: 'template-review', name: '정보성 식당 리뷰' },
    templateAnswers: [{ label: '방문일', text: '9월 20일', enabled: true }],
    // The lock holds the brief's ticks too, so the post carries one.
    qualityRules: ['title_saturation'],
    storyline: { paragraphs: [{ text: '성수의 카페를 걸어 봅니다.', files: ['IMG_1.jpg'] }] },
  })
  // A measurement the post would show on ② if it were not published.
  const MEASURED: FakeQualityReading[] = [
    {
      metric: 'cross_post_phrases',
      verdict: 'within_band',
      minimum: 3,
      publishedCount: 4,
      values: { share: 0.05, shareWarnAbove: 0.1 },
    },
  ]
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
    ],
    selections: [
      { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'seer' },
      { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
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
      quality: {
        accounts: { [published.slug]: M1_OVER_BAND },
        measurements: { [published.slug]: MEASURED },
      },
    })
  }

  it('lands on 글 완성 with 기억으로 저장 and export available, and the line reads 발행됨', async () => {
    renderPublished()

    const memories = await screen.findByRole('button', { name: '기억으로 저장' })
    selected('완성')
    // The same as a finalized post: publishing keeps the canonical content it extracts from.
    await waitFor(() => expect(memories).toBeEnabled())
    expect(screen.getByRole('heading', { name: '내보내기' })).toBeInTheDocument()
    await waitFor(() => expect(statusLine()).toHaveTextContent('발행됨'))
  })

  it('reads ② as prose under the one sentence, with only the road onward in its dock', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderPublished(calls)
    await screen.findByRole('button', { name: '기억으로 저장' })

    await openStep(user, '다듬기')
    const notice = await screen.findByText(LOCKED)
    expect(notice.closest('[role="status"]')).not.toBeNull()
    const prose = screen.getByRole('article', { name: '생성된 글' })
    expect(within(prose).getByRole('heading', { name: '비 온 뒤의 제주' })).toBeInTheDocument()
    expect(within(prose).getByText('비가 그치기를 기다렸다.')).toBeInTheDocument()
    // Read-only source inspection stays available; every prose action opens an explanation.
    // No editing control or measurement row is read or shown.
    const sourceActions = within(prose).getAllByRole('button')
    for (const action of sourceActions) expect(action).toHaveAttribute('aria-haspopup', 'dialog')
    await user.click(sourceActions[0]!)
    expect(await screen.findByRole('dialog', { name: '문구의 출처' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '닫기' }))
    expect(calls).not.toContain('SavePostContent')
    expect(screen.queryByRole('region', { name: '이 글의 측정값' })).toBeNull()
    expect(calls.filter((call) => call === 'GetPostMeasurement')).toHaveLength(0)
    // No block editor: nothing edits or moves a block, and nothing can start a content save.
    expect(screen.queryByRole('button', { name: '제목과 요약, 태그 수정' })).toBeNull()
    expect(screen.queryByRole('button', { name: '문단 추가' })).toBeNull()
    expect(screen.queryByRole('button', { name: /번째 블록/ })).toBeNull()

    // The step bar every step shares: the way back and the road onward, and nothing to revise.
    const dock = screen.getByLabelText('글 작업')
    const onward = within(dock).getByRole('button', { name: '글 완성으로 가기' })
    expect(within(dock).getAllByRole('button')).toEqual([
      within(dock).getByRole('button', { name: '이전' }),
      onward,
    ])
    expect(screen.queryByLabelText('수정 요청을 입력하세요')).toBeNull()

    await user.click(onward)
    await waitFor(() => selected('완성'))
    expect(calls.filter((call) => WRITES.includes(call))).toEqual([])
  })

  // POST-86: ② and ③ read as ④ does — the one sentence, nothing editable, and a bar holding only
  // the way back and the road onward.
  it.each([
    ['관찰', '스토리라인 먼저'],
    ['스토리라인', '다시 만들기'],
  ])('reads %s read-only, with only the road onward in its dock', async (step, action) => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderPublished(calls)
    await screen.findByRole('button', { name: '기억으로 저장' })

    await openStep(user, step)
    const notice = await screen.findByText(LOCKED)
    expect(notice.closest('[role="status"]')).not.toBeNull()
    const dock = screen.getByLabelText('글 작업')
    const onward = within(dock).getByRole('button', { name: '글 완성으로 가기' })
    expect(within(dock).getAllByRole('button')).toEqual([
      within(dock).getByRole('button', { name: '이전' }),
      onward,
    ])
    expect(screen.queryByRole('button', { name: action })).toBeNull()
    expect(screen.queryByRole('textbox', { name: '스토리라인 수정 요청' })).toBeNull()
    expect(screen.queryByRole('textbox', { name: /번째 문단/ })).toBeNull()

    await user.click(onward)
    await waitFor(() => selected('완성'))
    expect(calls.filter((call) => WRITES.includes(call))).toEqual([])
  })

  // POST-126: a published post's observations read as values, with no 다시 관찰하기 and no field.
  it('reads ②’s observations without a field or 다시 관찰하기', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${published.slug}`, {
      user: USER,
      calls,
      posts: { calls, posts: [{ ...published, observations: [OBSERVATION_FIXTURE] }] },
      providers: PROVIDERS,
    })
    await screen.findByRole('button', { name: '기억으로 저장' })

    await openStep(user, '관찰')
    expect(await screen.findByText('비가 그친 바닷가')).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: '장면' })).toBeNull()
    expect(screen.queryByRole('button', { name: '다시 관찰하기' })).toBeNull()
    expect(calls.filter((call) => WRITES.includes(call))).toEqual([])
  })

  it('shows ① read-only, says why once and sends no write', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    renderPublished(calls)
    await screen.findByRole('button', { name: '기억으로 저장' })

    await openStep(user, '생성')
    // Visit the material screens to check their published locks.
    await guideTo(user, GUIDE.details)
    const title = await screen.findByRole('textbox', { name: '제목' })
    expect(title).toHaveAttribute('readonly')
    expect(title).toHaveValue('성수 카페 투어')
    // Typing into a read-only field changes nothing, so nothing is queued.
    await user.type(title, ' 후기')
    expect(title).toHaveValue('성수 카페 투어')
    expect(await screen.findByLabelText('방문일')).toBeDisabled()
    expect(screen.getByLabelText('방문일')).toHaveValue('9월 20일')
    expect(screen.getByRole('switch', { name: '총평 별점 넣기' })).toBeDisabled()

    await guideTo(user, GUIDE.story)
    const memo = screen.getByRole('textbox', { name: '메모' })
    expect(memo).toHaveAttribute('readonly')
    expect(memo).toHaveValue('라떼가 맛있었다')
    await user.type(memo, ' 또')
    expect(memo).toHaveValue('라떼가 맛있었다')

    await guideTo(user, GUIDE.photos)
    // The picker's two file inputs, each labelled by its button-styled label.
    expect(screen.getByLabelText('사진·영상 추가')).toBeDisabled()
    expect(screen.getByLabelText('촬영')).toBeDisabled()
    expect(screen.getByRole('img', { name: 'IMG_1.jpg' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /삭제$/ })).toBeNull()
    for (const card of within(await templateField(user)).getAllByRole('button'))
      expect(card).toBeDisabled()
    expect(await voiceField(user)).toBeDisabled()

    // ①'s 확인 bar holds 이전 and 다음 only; the writing actions are ②'s, and a published post's ②
    // offers none of them.
    const dock = screen.getByLabelText('글 작업')
    await waitFor(() => expect(within(dock).getByRole('button', { name: '다음' })).toBeEnabled())
    expect(within(dock).queryByRole('button', { name: '바로 글 쓰기' })).toBeNull()
    expect(within(dock).queryByRole('button', { name: '스토리라인 먼저' })).toBeNull()
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
    expect(within(brief).getByLabelText('최소 태그 수')).toBeDisabled()
    expect(within(brief).getByRole('checkbox', { name: '기억 사용' })).toBeDisabled()
    for (const chip of within(within(brief).getByRole('group', { name: '분야' })).getAllByRole(
      'button',
    ))
      expect(chip).toBeDisabled()
    expect(within(brief).getByRole('button', { name: '저장' })).toBeDisabled()
    // The post's ticked M1 row stays checked and holds still.
    const m1 = await within(brief).findByRole('checkbox', { name: /^제목 도배율 42%/ })
    expect(m1).toBeDisabled()
    expect(m1).toBeChecked()

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
        posts: [finalizedPostRow({ slug, title: '제주', memo: '갔다' })],
        // Published from another tab between two autosaves.
        publishOnDraftSave: slug,
      },
    })

    await openStep(user, '생성')
    await user.type(await screen.findByLabelText('제목'), ' 여행기')
    await waitFor(() => expect(calls).toContain('SavePostDraft'), { timeout: 4_000 })

    // The refusal refetches the post, which now reads published and carries the reader to ③.
    await waitFor(() => expect(statusLine()).toHaveTextContent('발행됨'), { timeout: 4_000 })
    await waitFor(() => selected('완성'))
    expect(screen.queryByText('저장하지 못했어요 · 다시 시도 중')).toBeNull()

    // Past two retry windows: the save went out once and was never retried.
    await new Promise((resolve) => setTimeout(resolve, 3_000))
    expect(calls.filter((call) => call === 'SavePostDraft')).toHaveLength(1)
    expect(screen.queryByText('저장하지 못했어요 · 다시 시도 중')).toBeNull()

    await openStep(user, '생성')
    const title = await screen.findByLabelText('제목')
    expect(title).toHaveAttribute('readonly')
    // The server's, not the text the lock refused.
    expect(title).toHaveValue('제주')
    expect(screen.getAllByText(LOCKED)).toHaveLength(1)
  })

  // Reopening is clearing the post's URL on 글 완성 (`published → finalized`). The fields start
  // again from the server's, not from the text the lock refused.
  it('reseeds ① from the server when the post reopens', async () => {
    const calls: string[] = []
    const user = userEvent.setup()
    const slug = '20260820-jeju'
    const finalized = finalizedPostRow({ slug, title: '제주', memo: '갔다' })
    const { queryClient } = renderAppAt(`/posts/${slug}`, {
      user: USER,
      calls,
      posts: {
        calls,
        posts: [finalized],
        getSequence: [
          finalized,
          publishedPostRow({ slug, title: '제주', memo: '갔다' }),
          finalized,
        ],
        publishOnDraftSave: slug,
      },
    })

    await openStep(user, '생성')
    await user.type(await screen.findByLabelText('제목'), ' 여행기')
    await user.type(screen.getByLabelText('메모'), ' 또')
    await waitFor(() => expect(statusLine()).toHaveTextContent('발행됨'), { timeout: 4_000 })

    // The URL was cleared elsewhere: the next read finds the post finalized again.
    await queryClient.invalidateQueries()
    await waitFor(() => expect(statusLine()).toHaveTextContent('확정'))
    await openStep(user, '생성')
    const title = await screen.findByLabelText('제목')
    expect(title).not.toHaveAttribute('readonly')
    expect(title).toHaveValue('제주')
    expect(screen.getByLabelText('메모')).toHaveValue('갔다')
    expect(screen.queryByText(LOCKED)).toBeNull()
  })
})
