import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakePurposeRow, FakePurposesOptions } from '@/test/purposes'

const USER = { id: 'alice' }

const PURPOSES: FakePurposeRow[] = [
  {
    id: 'purpose-review',
    name: '정보성 식당 리뷰',
    description: '협찬 방문 리뷰',
    instructions: '사진마다 설명',
    postCount: 2,
  },
  { id: 'purpose-diary', name: '일기', instructions: '그날 있었던 일' },
]

function renderPurposes(purposes: FakePurposesOptions = {}, calls: string[] = []) {
  return renderAppAt('/purposes', {
    user: USER,
    calls,
    purposes: { purposes: PURPOSES, ...purposes },
  })
}

const section = async (name: string) => within(await screen.findByRole('region', { name }))

/** One row of the directory, by the purpose's name. Every row carries the same three field
 *  labels and the same pencils, so a query has to be scoped to a row to mean anything. */
async function row(name: string) {
  const list = await section('저장된 용도')
  const found = list
    .getAllByRole('listitem')
    .find((item) => within(item).queryByText(name) !== null)
  if (!found) throw new Error(`no row named ${name}`)
  return within(found)
}

describe('the purpose directory', () => {
  // Plan 11 A11/A13: the screen reads and edits authored text and nothing else.
  it('lists the account purposes with their post counts and asks no model anything', async () => {
    const calls: string[] = []
    renderPurposes({}, calls)

    expect(await screen.findByRole('heading', { level: 1, name: '용도' })).toBeInTheDocument()
    const list = await section('저장된 용도')
    expect(list.getByText('정보성 식당 리뷰')).toBeInTheDocument()
    expect(list.getByText('협찬 방문 리뷰')).toBeInTheDocument()
    expect(list.getByText('글 2개')).toBeInTheDocument()
    expect(list.getByText('글 0개')).toBeInTheDocument()

    // Mounting the screen starts no job and calls no provider ([I5]).
    expect(calls.filter((call) => call !== 'GetMe' && call !== 'ListPurposes')).toEqual([])
  })

  // Plan 11 A11: the worked example is copy. Nothing is created for the user.
  it('shows the example as text and creates no row when the account has none', async () => {
    const calls: string[] = []
    renderPurposes({ purposes: [] }, calls)

    const empty = await section('아직 저장된 용도가 없어요')
    expect(empty.getByText('정보성 식당 리뷰')).toBeInTheDocument()
    // A row would be a link or a delete button; the example is neither.
    expect(empty.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '저장된 용도' })).not.toBeInTheDocument()
    expect(calls.filter((call) => call === 'CreatePurpose')).toEqual([])
  })

  it('creates a purpose from the three fields and shows it in the list', async () => {
    const user = userEvent.setup()
    renderPurposes({ purposes: [] })
    const form = await section('새 용도')

    await user.type(form.getByLabelText('용도 이름'), '정보성 식당 리뷰')
    await user.type(form.getByLabelText(/어떤 글인가요/), '협찬 방문 리뷰')
    await user.type(form.getByLabelText('작성 지침'), '사진마다 무엇인지 설명하세요.')
    await user.click(form.getByRole('button', { name: '용도 만들기' }))

    const list = await section('저장된 용도')
    await waitFor(() => expect(list.getByText('정보성 식당 리뷰')).toBeInTheDocument())
    // Only what was submitted is cleared.
    expect(form.getByLabelText('용도 이름')).toHaveValue('')
  })

  it('refuses a duplicate name with the server message and keeps what was typed', async () => {
    const user = userEvent.setup()
    renderPurposes()
    const form = await section('새 용도')

    await user.type(form.getByLabelText('용도 이름'), '일기')
    await user.type(form.getByLabelText('작성 지침'), '지침')
    await user.click(form.getByRole('button', { name: '용도 만들기' }))

    expect(await screen.findByText('같은 이름의 용도가 이미 있어요')).toBeInTheDocument()
    expect(form.getByLabelText('용도 이름')).toHaveValue('일기')
  })

  // Plan 11 A2: the edit unit is one field. The request must carry only that field.
  it('edits one field at a time and sends only the field that changed', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakePurposesOptions['updates']> = []
    renderPurposes({ updates })

    const review = await row('정보성 식당 리뷰')
    await user.click(review.getByRole('button', { name: '작성 지침 수정' }))
    const editor = review.getByLabelText('작성 지침')
    await user.clear(editor)
    await user.type(editor, '사진마다 무엇인지 쓰세요')
    await user.click(review.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toEqual({
      id: 'purpose-review',
      name: undefined,
      description: undefined,
      instructions: '사진마다 무엇인지 쓰세요',
    })
    // Read-first again once the save lands.
    await waitFor(() =>
      expect(review.getByRole('button', { name: '작성 지침 수정' })).toBeInTheDocument(),
    )
  })

  it('keeps the draft and the editor open when a save is refused', async () => {
    const user = userEvent.setup()
    renderPurposes()

    const diary = await row('일기')
    await user.click(diary.getByRole('button', { name: '이름 수정' }))
    const editor = diary.getByLabelText('이름')
    await user.clear(editor)
    await user.type(editor, '정보성 식당 리뷰')
    await user.click(diary.getByRole('button', { name: '저장' }))

    expect(await diary.findByText('같은 이름의 용도가 이미 있어요')).toBeInTheDocument()
    expect(diary.getByLabelText('이름')).toHaveValue('정보성 식당 리뷰')
  })

  // Plan 11 A9: the confirmation states the count, and the delete detaches rather than cascading.
  it('names how many posts lose their purpose before deleting it', async () => {
    const user = userEvent.setup()
    renderPurposes()

    const review = await row('정보성 식당 리뷰')
    await user.click(review.getByRole('button', { name: '정보성 식당 리뷰 삭제' }))

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/2개의 글에서 용도가 해제됩니다/)).toBeInTheDocument()
    expect(dialog.getByText(/글과 본문은 그대로 남아요/)).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '삭제' }))

    await waitFor(() => expect(screen.queryByText('정보성 식당 리뷰')).not.toBeInTheDocument())
  })

  it('says so plainly when nothing references the purpose', async () => {
    const user = userEvent.setup()
    renderPurposes()

    const diary = await row('일기')
    await user.click(diary.getByRole('button', { name: '일기 삭제' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/이 용도를 쓰는 글이 없어요/)).toBeInTheDocument()
  })

  // A rename changes what every post DISPLAYS, so the list has to be re-read. Without it the
  // badges keep the old name for as long as the post queries stay fresh.
  it('re-reads the posts after a rename, because they carry the projected name', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderPurposes({}, calls)

    const diary = await row('일기')
    await user.click(diary.getByRole('button', { name: '이름 수정' }))
    const editor = diary.getByLabelText('이름')
    await user.clear(editor)
    await user.type(editor, '여행일지')
    await user.click(diary.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(calls).toContain('UpdatePurpose'))
    // Invalidation is what a mounted list would act on; here it is enough that the keys were
    // marked stale, which the fetch below proves for the one query this screen can observe.
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ListPurposes').length).toBeGreaterThan(1),
    )
  })

  // post_count is a projection over POSTS, so no purpose mutation can invalidate it. The
  // screen re-reads the directory on every mount instead — the count is what the user
  // confirms a destructive detach against.
  it('re-reads the directory on mount rather than serving a cached post count', async () => {
    const calls: string[] = []
    const { queryClient, transport } = renderPurposes({}, calls)
    await screen.findByRole('heading', { level: 1, name: '용도' })
    await waitFor(() => expect(calls.filter((call) => call === 'ListPurposes')).toHaveLength(1))

    // A cached entry that is merely fresh must not be trusted for this screen.
    const cached = queryClient.getQueryData(['purposes', transport, USER.id])
    expect(cached).toBeDefined()
    expect(queryClient.getQueryState(['purposes', transport, USER.id])?.isInvalidated).toBe(false)
    const options = queryClient.getQueryCache().find({ queryKey: ['purposes', transport, USER.id] })
      ?.options as { staleTime?: number; refetchOnMount?: unknown }
    expect(options.staleTime).toBe(0)
    expect(options.refetchOnMount).toBe('always')
  })

  it('offers a retry when the directory cannot be read', async () => {
    renderPurposes({ listFails: true })
    expect(await screen.findByText('용도 목록을 불러오지 못했어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
})
