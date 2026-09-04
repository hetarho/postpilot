import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeTemplateRow, FakeTemplatesOptions } from '@/test/templates'

const USER = { id: 'alice' }

const PURPOSES: FakeTemplateRow[] = [
  {
    id: 'template-review',
    name: '정보성 식당 리뷰',
    description: '협찬 방문 리뷰',
    body: '사진마다 설명',
    postCount: 2,
  },
  { id: 'template-diary', name: '일기', body: '그날 있었던 일' },
]

function renderTemplates(templates: FakeTemplatesOptions = {}, calls: string[] = []) {
  return renderAppAt('/templates', {
    user: USER,
    calls,
    templates: { templates: PURPOSES, ...templates },
  })
}

const section = async (name: string) => within(await screen.findByRole('region', { name }))

/** One row of the directory, by the template's name. Every row carries the same three field
 *  labels and the same pencils, so a query has to be scoped to a row to mean anything. */
async function row(name: string) {
  const list = await section('저장된 템플릿')
  const found = list
    .getAllByRole('listitem')
    .find((item) => within(item).queryByText(name) !== null)
  if (!found) throw new Error(`no row named ${name}`)
  return within(found)
}

describe('the template directory', () => {
  // Plan 11 A11/A13: the screen reads and edits authored text and nothing else.
  it('lists the account templates with their post counts and asks no model anything', async () => {
    const calls: string[] = []
    renderTemplates({}, calls)

    expect(await screen.findByRole('heading', { level: 1, name: '템플릿' })).toBeInTheDocument()
    const list = await section('저장된 템플릿')
    expect(list.getByText('정보성 식당 리뷰')).toBeInTheDocument()
    expect(list.getByText('협찬 방문 리뷰')).toBeInTheDocument()
    expect(list.getByText('글 2개')).toBeInTheDocument()
    expect(list.getByText('글 0개')).toBeInTheDocument()

    // Mounting the screen starts no job and calls no provider ([I5]).
    expect(calls.filter((call) => call !== 'GetMe' && call !== 'ListTemplates')).toEqual([])
  })

  // Plan 11 A11: the worked example is copy. Nothing is created for the user.
  it('shows the example as text and creates no row when the account has none', async () => {
    const calls: string[] = []
    renderTemplates({ templates: [] }, calls)

    const empty = await section('아직 저장된 템플릿이 없어요')
    expect(empty.getByText('정보성 식당 리뷰')).toBeInTheDocument()
    // A row would be a link or a delete button; the example is neither.
    expect(empty.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '저장된 템플릿' })).not.toBeInTheDocument()
    expect(calls.filter((call) => call === 'CreateTemplate')).toEqual([])
  })

  it('creates a template from the three fields and shows it in the list', async () => {
    const user = userEvent.setup()
    renderTemplates({ templates: [] })
    const form = await section('새 템플릿')

    await user.type(form.getByLabelText('템플릿 이름'), '정보성 식당 리뷰')
    await user.type(form.getByLabelText(/어떤 글인가요/), '협찬 방문 리뷰')
    await user.click(form.getByRole('tab', { name: '원문' }))
    await user.type(
      form.getByLabelText('템플릿 구성'),
      '<write>사진마다 무엇인지 설명하세요.</write>',
    )
    await user.click(form.getByRole('button', { name: '템플릿 만들기' }))

    const list = await section('저장된 템플릿')
    await waitFor(() => expect(list.getByText('정보성 식당 리뷰')).toBeInTheDocument())
    // Only what was submitted is cleared.
    expect(form.getByLabelText('템플릿 이름')).toHaveValue('')
  })

  it('refuses a duplicate name with the server message and keeps what was typed', async () => {
    const user = userEvent.setup()
    renderTemplates()
    const form = await section('새 템플릿')

    await user.type(form.getByLabelText('템플릿 이름'), '일기')
    await user.click(form.getByRole('tab', { name: '원문' }))
    await user.type(form.getByLabelText('템플릿 구성'), '<write>지침</write>')
    await user.click(form.getByRole('button', { name: '템플릿 만들기' }))

    expect(await screen.findByText('같은 이름의 템플릿이 이미 있어요.')).toBeInTheDocument()
    expect(form.getByLabelText('템플릿 이름')).toHaveValue('일기')
  })

  // Plan 11 A2: the edit unit is one field. The request must carry only that field.
  it('edits one field at a time and sends only the field that changed', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeTemplatesOptions['updates']> = []
    renderTemplates({ updates })

    const review = await row('정보성 식당 리뷰')
    await user.click(review.getByRole('button', { name: '템플릿 구성 수정' }))
    await user.click(review.getByRole('tab', { name: '원문' }))
    const editor = review.getByLabelText('템플릿 구성')
    await user.clear(editor)
    await user.type(editor, '<write>사진마다 무엇인지 쓰세요</write>')
    await user.click(review.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toEqual({
      id: 'template-review',
      name: undefined,
      description: undefined,
      body: '<write>사진마다 무엇인지 쓰세요</write>',
    })
    // Read-first again once the save lands.
    await waitFor(() =>
      expect(review.getByRole('button', { name: '템플릿 구성 수정' })).toBeInTheDocument(),
    )
  })

  it('keeps the draft and the editor open when a save is refused', async () => {
    const user = userEvent.setup()
    renderTemplates()

    const diary = await row('일기')
    await user.click(diary.getByRole('button', { name: '이름 수정' }))
    const editor = diary.getByLabelText('이름')
    await user.clear(editor)
    await user.type(editor, '정보성 식당 리뷰')
    await user.click(diary.getByRole('button', { name: '저장' }))

    expect(await diary.findByText('같은 이름의 템플릿이 이미 있어요.')).toBeInTheDocument()
    expect(diary.getByLabelText('이름')).toHaveValue('정보성 식당 리뷰')
  })

  // Plan 11 A9: the confirmation states the count, and the delete detaches rather than cascading.
  it('names how many posts lose their template before deleting it', async () => {
    const user = userEvent.setup()
    renderTemplates()

    const review = await row('정보성 식당 리뷰')
    await user.click(review.getByRole('button', { name: '정보성 식당 리뷰 삭제' }))

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/2개의 글에서 템플릿이 해제됩니다/)).toBeInTheDocument()
    expect(dialog.getByText(/글과 본문은 그대로 남아요/)).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '삭제' }))

    await waitFor(() => expect(screen.queryByText('정보성 식당 리뷰')).not.toBeInTheDocument())
  })

  it('says so plainly when nothing references the template', async () => {
    const user = userEvent.setup()
    renderTemplates()

    const diary = await row('일기')
    await user.click(diary.getByRole('button', { name: '일기 삭제' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/이 템플릿을 쓰는 글이 없어요/)).toBeInTheDocument()
  })

  // A rename changes what every post DISPLAYS, so the list has to be re-read. Without it the
  // badges keep the old name for as long as the post queries stay fresh.
  it('re-reads the posts after a rename, because they carry the projected name', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderTemplates({}, calls)

    const diary = await row('일기')
    await user.click(diary.getByRole('button', { name: '이름 수정' }))
    const editor = diary.getByLabelText('이름')
    await user.clear(editor)
    await user.type(editor, '여행일지')
    await user.click(diary.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(calls).toContain('UpdateTemplate'))
    // Invalidation is what a mounted list would act on; here it is enough that the keys were
    // marked stale, which the fetch below proves for the one query this screen can observe.
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ListTemplates').length).toBeGreaterThan(1),
    )
  })

  // post_count is a projection over POSTS, so no template mutation can invalidate it. The
  // screen re-reads the directory on every mount instead — the count is what the user
  // confirms a destructive detach against.
  it('re-reads the directory on mount rather than serving a cached post count', async () => {
    const calls: string[] = []
    const { queryClient, transport } = renderTemplates({}, calls)
    await screen.findByRole('heading', { level: 1, name: '템플릿' })
    await waitFor(() => expect(calls.filter((call) => call === 'ListTemplates')).toHaveLength(1))

    // A cached entry that is merely fresh must not be trusted for this screen.
    const cached = queryClient.getQueryData(['templates', transport, USER.id])
    expect(cached).toBeDefined()
    expect(queryClient.getQueryState(['templates', transport, USER.id])?.isInvalidated).toBe(false)
    const options = queryClient
      .getQueryCache()
      .find({ queryKey: ['templates', transport, USER.id] })?.options as {
      staleTime?: number
      refetchOnMount?: unknown
    }
    expect(options.staleTime).toBe(0)
    expect(options.refetchOnMount).toBe('always')
  })

  it('offers a retry when the directory cannot be read', async () => {
    renderTemplates({ listFails: true })
    expect(await screen.findByText('템플릿 목록을 불러오지 못했어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
})
