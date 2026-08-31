import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoGuidelineScope } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeGuidelineRow, FakeGuidelinesOptions } from '@/test/guidelines'
import type { FakePurposeRow } from '@/test/purposes'

const USER = { id: 'alice' }

const PURPOSES: FakePurposeRow[] = [
  { id: 'purpose-review', name: '무인가게 리뷰' },
  { id: 'purpose-sponsored', name: '협찬 리뷰' },
]

const GUIDELINES: FakeGuidelineRow[] = [
  { id: 'guideline-global', text: '없는 사실을 쓰지 않기' },
  {
    id: 'guideline-scoped',
    text: 'CCTV를 언급하지 않기',
    purposeRefs: [{ id: 'purpose-review', name: '무인가게 리뷰' }],
  },
  // Every purpose it named was deleted: a real state, not a missing value.
  { id: 'guideline-orphan', text: '주인 이야기를 쓰지 않기', scope: 'purposes', purposeRefs: [] },
]

function renderGuidelines(guidelines: FakeGuidelinesOptions = {}, calls: string[] = []) {
  return renderAppAt('/guidelines', {
    user: USER,
    calls,
    purposes: { purposes: PURPOSES },
    guidelines: { guidelines: GUIDELINES, ...guidelines },
  })
}

const section = async (name: string) => within(await screen.findByRole('region', { name }))

/** One row of the list, by its text. Every row carries the same pencils, so a query has to be
 *  scoped to a row to mean anything. */
async function row(text: string) {
  const list = await section('저장된 지침')
  const found = list
    .getAllByRole('listitem')
    .find((item) => within(item).queryByText(text) !== null)
  if (!found) throw new Error(`no row with text ${text}`)
  return within(found)
}

describe('the guideline list', () => {
  // A14/A15: the screen reads and edits authored text and nothing else.
  it('lists the guidelines in injection order with their scope and asks no model anything', async () => {
    const calls: string[] = []
    renderGuidelines({}, calls)

    expect(await screen.findByRole('heading', { level: 1, name: '지침' })).toBeInTheDocument()
    const list = await section('저장된 지침')
    const items = list.getAllByRole('listitem')
    // The server's order IS the injection order: global group first, then scoped.
    expect(items).toHaveLength(3)
    expect(within(items[0]).getByText('없는 사실을 쓰지 않기')).toBeInTheDocument()
    expect(within(items[0]).getByText('전역')).toBeInTheDocument()
    expect(within(items[1]).getByText('CCTV를 언급하지 않기')).toBeInTheDocument()
    expect(within(items[1]).getByText('무인가게 리뷰')).toBeInTheDocument()
    // A12: an orphaned scope says so in words, not by colour alone.
    expect(within(items[2]).getByText('적용 대상 없음')).toBeInTheDocument()

    // A15: mounting the screen starts no job and calls no provider ([I5]).
    const allowed = ['GetMe', 'ListGuidelines', 'ListPurposes']
    expect(calls.filter((call) => !allowed.includes(call))).toEqual([])
  })

  // A14: the worked example is copy. Nothing is created for the user.
  it('shows the example as text and creates no row when the account has none', async () => {
    const calls: string[] = []
    renderGuidelines({ guidelines: [] }, calls)

    const empty = await section('아직 저장된 지침이 없어요')
    expect(
      empty.getByText('무인 매장 글에서 직원·주인과의 상호작용이나 CCTV를 언급하지 않기'),
    ).toBeInTheDocument()
    // A row would carry a delete button; the example carries none.
    expect(empty.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '저장된 지침' })).not.toBeInTheDocument()
    expect(calls.filter((call) => call === 'CreateGuideline')).toEqual([])
  })

  it('creates a global guideline and shows it in the list', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
    renderGuidelines({ guidelines: [], creates })
    const form = await section('새 지침')

    await user.type(form.getByLabelText('지침'), '가격을 지어내지 않기')
    await user.click(form.getByRole('button', { name: '지침 만들기' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    // 전역 is the default, and a global scope carries no purpose ids.
    expect(creates[0]).toEqual({
      text: '가격을 지어내지 않기',
      scope: ProtoGuidelineScope.GLOBAL,
      purposeIds: [],
    })
    const list = await section('저장된 지침')
    await waitFor(() => expect(list.getByText('가격을 지어내지 않기')).toBeInTheDocument())
    // Only what was submitted is cleared.
    expect(form.getByLabelText('지침')).toHaveValue('')
  })

  // A2/A14: a scoped create must name at least one owned purpose, picked from the directory.
  it('creates a purpose-scoped guideline from the scope control', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
    renderGuidelines({ guidelines: [], creates })
    const form = await section('새 지침')

    await user.type(form.getByLabelText('지침'), '협찬 표기를 빠뜨리지 않기')
    await user.click(form.getByRole('tab', { name: '특정 용도' }))
    // The submit is unreachable until a purpose is picked: `purposes` with no ids is the shape
    // the server refuses.
    expect(form.getByRole('button', { name: '지침 만들기' })).toBeDisabled()
    await user.click(form.getByLabelText('협찬 리뷰'))
    await user.click(form.getByRole('button', { name: '지침 만들기' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toEqual({
      text: '협찬 표기를 빠뜨리지 않기',
      scope: ProtoGuidelineScope.PURPOSES,
      purposeIds: ['purpose-sponsored'],
    })
  })

  it('refuses a duplicate text with the server message and keeps what was typed', async () => {
    const user = userEvent.setup()
    renderGuidelines()
    const form = await section('새 지침')

    await user.type(form.getByLabelText('지침'), '없는 사실을 쓰지 않기')
    await user.click(form.getByRole('button', { name: '지침 만들기' }))

    expect(await screen.findByText('이미 같은 지침이 있어요.')).toBeInTheDocument()
    expect(form.getByLabelText('지침')).toHaveValue('없는 사실을 쓰지 않기')
  })

  // A3: the edit unit is one part. A text edit must carry no scope.
  it('edits the text read-first and sends only the text', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeGuidelinesOptions['updates']> = []
    renderGuidelines({ updates })

    const scoped = await row('CCTV를 언급하지 않기')
    await user.click(scoped.getByRole('button', { name: '지침 수정' }))
    const editor = scoped.getByLabelText('지침')
    await user.clear(editor)
    await user.type(editor, 'CCTV와 보안 이야기를 쓰지 않기')
    await user.click(scoped.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toEqual({
      id: 'guideline-scoped',
      text: 'CCTV와 보안 이야기를 쓰지 않기',
      scope: undefined,
    })
    // Read-first again once the save lands.
    await waitFor(() =>
      expect(scoped.getByRole('button', { name: '지침 수정' })).toBeInTheDocument(),
    )
  })

  // A3: a scope patch replaces the whole scope, and carries no text.
  it('replaces the whole scope in one patch and sends no text', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeGuidelinesOptions['updates']> = []
    renderGuidelines({ updates })

    const global = await row('없는 사실을 쓰지 않기')
    await user.click(global.getByRole('button', { name: '적용 범위 수정' }))
    await user.click(global.getByRole('tab', { name: '특정 용도' }))
    await user.click(global.getByLabelText('무인가게 리뷰'))
    await user.click(global.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toEqual({
      id: 'guideline-global',
      text: undefined,
      scope: { scope: ProtoGuidelineScope.PURPOSES, purposeIds: ['purpose-review'] },
    })
  })

  // A12: an orphaned scoped guideline can be rescoped back into use.
  it('lets an orphaned guideline be rescoped', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeGuidelinesOptions['updates']> = []
    renderGuidelines({ updates })

    const orphan = await row('주인 이야기를 쓰지 않기')
    expect(orphan.getByText('적용 대상 없음')).toBeInTheDocument()
    await user.click(orphan.getByRole('button', { name: '적용 범위 수정' }))
    await user.click(orphan.getByRole('tab', { name: '전역' }))
    await user.click(orphan.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]?.scope).toEqual({ scope: ProtoGuidelineScope.GLOBAL, purposeIds: [] })
  })

  it('keeps the draft and the editor open when a save is refused', async () => {
    const user = userEvent.setup()
    renderGuidelines()

    const scoped = await row('CCTV를 언급하지 않기')
    await user.click(scoped.getByRole('button', { name: '지침 수정' }))
    const editor = scoped.getByLabelText('지침')
    await user.clear(editor)
    await user.click(scoped.getByRole('button', { name: '저장' }))

    // An empty text cannot even be submitted, so the editor is simply still open with the draft.
    expect(scoped.getByRole('button', { name: '저장' })).toBeDisabled()
    expect(scoped.getByLabelText('지침')).toHaveValue('')
  })

  // A14: the confirmation states what a delete does and does not touch. There is no count.
  it('states that enqueued work keeps its frozen text before deleting', async () => {
    const user = userEvent.setup()
    renderGuidelines()

    const global = await row('없는 사실을 쓰지 않기')
    await user.click(global.getByRole('button', { name: '지침 삭제' }))

    const dialog = within(await screen.findByRole('dialog'))
    expect(
      dialog.getByText(/이미 시작된 AI 작업은 시작할 때의 지침으로 끝나고/),
    ).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '삭제' }))

    await waitFor(() => expect(screen.queryByText('없는 사실을 쓰지 않기')).not.toBeInTheDocument())
  })

  // The purpose names on each chip are a PROJECTION, so the list is re-read on every mount
  // rather than served from a merely-fresh cache entry.
  it('re-reads the list on mount rather than trusting a fresh cache entry', async () => {
    const calls: string[] = []
    const { queryClient, transport } = renderGuidelines({}, calls)
    await screen.findByRole('heading', { level: 1, name: '지침' })
    await waitFor(() => expect(calls.filter((call) => call === 'ListGuidelines')).toHaveLength(1))
    expect(queryClient.getQueryData(['guidelines', transport, USER.id])).toBeDefined()
  })

  it('offers a retry when the list cannot be read', async () => {
    renderGuidelines({ listFails: true })
    expect(await screen.findByText('지침 목록을 불러오지 못했어요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
})
