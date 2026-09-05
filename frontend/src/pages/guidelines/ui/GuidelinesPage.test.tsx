import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoGuidelineScope } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeGuidelineRow, FakeGuidelinesOptions } from '@/test/guidelines'
import type { FakeTemplateRow } from '@/test/templates'

const USER = { id: 'alice' }

const PURPOSES: FakeTemplateRow[] = [
  { id: 'template-review', name: '무인가게 리뷰' },
  { id: 'template-sponsored', name: '협찬 리뷰' },
]

const GUIDELINES: FakeGuidelineRow[] = [
  { id: 'guideline-global', text: '없는 사실을 쓰지 않기' },
  {
    id: 'guideline-scoped',
    text: 'CCTV를 언급하지 않기',
    templateRefs: [{ id: 'template-review', name: '무인가게 리뷰' }],
  },
  // Every template it named was deleted: a real state, not a missing value.
  { id: 'guideline-orphan', text: '주인 이야기를 쓰지 않기', scope: 'templates', templateRefs: [] },
]

function renderGuidelines(guidelines: FakeGuidelinesOptions = {}, calls: string[] = []) {
  return renderAppAt('/guidelines', {
    user: USER,
    calls,
    templates: { templates: PURPOSES },
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
    const allowed = ['GetMe', 'ListGuidelines', 'ListGuidelineCandidates', 'ListTemplates']
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
    // 전역 is the default, and a global scope carries no template ids.
    expect(creates[0]).toEqual({
      text: '가격을 지어내지 않기',
      scope: ProtoGuidelineScope.GLOBAL,
      templateIds: [],
    })
    const list = await section('저장된 지침')
    await waitFor(() => expect(list.getByText('가격을 지어내지 않기')).toBeInTheDocument())
    // Only what was submitted is cleared.
    expect(form.getByLabelText('지침')).toHaveValue('')
  })

  // A2/A14: a scoped create must name at least one owned template, picked from the directory.
  it('creates a template-scoped guideline from the scope control', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
    renderGuidelines({ guidelines: [], creates })
    const form = await section('새 지침')

    await user.type(form.getByLabelText('지침'), '협찬 표기를 빠뜨리지 않기')
    await user.click(form.getByRole('tab', { name: '특정 템플릿' }))
    // The submit is unreachable until a template is picked: `templates` with no ids is the shape
    // the server refuses.
    expect(form.getByRole('button', { name: '지침 만들기' })).toBeDisabled()
    await user.click(form.getByLabelText('협찬 리뷰'))
    await user.click(form.getByRole('button', { name: '지침 만들기' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toEqual({
      text: '협찬 표기를 빠뜨리지 않기',
      scope: ProtoGuidelineScope.TEMPLATES,
      templateIds: ['template-sponsored'],
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
    await user.click(global.getByRole('tab', { name: '특정 템플릿' }))
    await user.click(global.getByLabelText('무인가게 리뷰'))
    await user.click(global.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toEqual({
      id: 'guideline-global',
      text: undefined,
      scope: { scope: ProtoGuidelineScope.TEMPLATES, templateIds: ['template-review'] },
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
    expect(updates[0]?.scope).toEqual({ scope: ProtoGuidelineScope.GLOBAL, templateIds: [] })
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

  // The template names on each chip are a PROJECTION, so the list is re-read on every mount
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

/** The 후보 section (change 26). Every row is one instruction a completed revision recorded
 *  verbatim; nothing here is learned, and nothing reaches a prompt until it is approved. */
describe('the guideline candidate section', () => {
  const CANDIDATES = [
    // Given in the SERVER's review order: most-repeated first, then most recent.
    { id: 'candidate-repeated', text: '여기 너무 광고 같아', postSlug: 'post-1', occurrences: 5 },
    { id: 'candidate-once', text: '존댓말로 써줘', postSlug: 'post-2' },
    // The source post was deleted: the text survives, the link does not.
    { id: 'candidate-orphan', text: '문단을 짧게' },
  ]

  const candidateRow = async (text: string) => {
    const list = await section('후보 지침')
    const found = list
      .getAllByRole('listitem')
      .find((item) => within(item).queryByText(text) !== null)
    if (!found) throw new Error(`no candidate row with text ${text}`)
    return within(found)
  }

  // A1/A2: the review order and the occurrence count, and nothing asked of a model.
  it('lists the pending candidates in review order with their occurrence count', async () => {
    const calls: string[] = []
    renderGuidelines({ candidates: CANDIDATES }, calls)

    const list = await section('후보 지침')
    const items = list.getAllByRole('listitem')
    expect(items).toHaveLength(3)
    expect(within(items[0]).getByText('여기 너무 광고 같아')).toBeInTheDocument()
    expect(within(items[0]).getByText('5번 요청함')).toBeInTheDocument()
    expect(within(items[1]).getByText('존댓말로 써줘')).toBeInTheDocument()
    // A single sighting shows no count: the count exists to mark a repeat.
    expect(within(items[1]).queryByText('1번 요청함')).not.toBeInTheDocument()

    // A12: reading the section calls no provider and enqueues nothing ([I5]).
    const allowed = ['GetMe', 'ListGuidelines', 'ListGuidelineCandidates', 'ListTemplates']
    expect(calls.filter((call) => !allowed.includes(call))).toEqual([])
  })

  // A11: a deleted post leaves the candidate listed with its text and no link.
  it('names the source post as a link, or says it is gone', async () => {
    renderGuidelines({ candidates: CANDIDATES })

    const repeated = await candidateRow('여기 너무 광고 같아')
    expect(repeated.getByRole('link', { name: '요청한 글 보기' })).toHaveAttribute(
      'href',
      '/posts/post-1',
    )
    const orphan = await candidateRow('문단을 짧게')
    expect(orphan.queryByRole('link')).not.toBeInTheDocument()
    expect(orphan.getByText('요청한 글이 삭제됐어요')).toBeInTheDocument()
  })

  // A5: scope is chosen at APPROVAL, 전역 preselected, and the save goes through the standard
  // create RPC with exactly the chosen scope.
  it('approves through the create with the chosen scope and takes the row out of the section', async () => {
    const user = userEvent.setup()
    const creates: FakeGuidelinesOptions['creates'] = []
    renderGuidelines({ candidates: CANDIDATES, creates })

    const repeated = await candidateRow('여기 너무 광고 같아')
    await user.click(repeated.getByRole('button', { name: '승인' }))

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByLabelText('지침')).toHaveValue('여기 너무 광고 같아')
    // 전역 is preselected: a rule applies everywhere unless the user narrows it.
    expect(dialog.getByRole('tab', { name: '전역' })).toHaveAttribute('aria-selected', 'true')
    await user.click(dialog.getByRole('button', { name: '지침으로 저장' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toMatchObject({
      text: '여기 너무 광고 같아',
      scope: ProtoGuidelineScope.GLOBAL,
      templateIds: [],
    })
    // The approval names the row it approves rather than relying on the text match alone.
    expect(creates[0].fromCandidateId).toBe('candidate-repeated')
    // The approved candidate leaves the 후보 section and the rule appears in the saved list:
    // one create invalidated both, because an approval moves a row from one to the other.
    await waitFor(async () =>
      expect((await section('후보 지침')).getAllByRole('listitem')).toHaveLength(2),
    )
    const saved = await section('저장된 지침')
    expect(saved.getAllByRole('listitem')).toHaveLength(4)
    expect(saved.getByText('여기 너무 광고 같아')).toBeInTheDocument()
  })

  // A5 with a narrowed scope, and A7's edit path: an edited approval carries the candidate id,
  // because its text can no longer be matched.
  it('carries the candidate id and the narrowed scope when the text was edited first', async () => {
    const user = userEvent.setup()
    const creates: FakeGuidelinesOptions['creates'] = []
    renderGuidelines({ candidates: CANDIDATES, creates })

    const repeated = await candidateRow('여기 너무 광고 같아')
    await user.click(repeated.getByRole('button', { name: '승인' }))

    const dialog = within(await screen.findByRole('dialog'))
    await user.clear(dialog.getByLabelText('지침'))
    await user.type(dialog.getByLabelText('지침'), '광고처럼 읽히는 문장을 쓰지 않기')
    await user.click(dialog.getByRole('tab', { name: '특정 템플릿' }))
    await user.click(dialog.getByRole('checkbox', { name: '무인가게 리뷰' }))
    await user.click(dialog.getByRole('button', { name: '지침으로 저장' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toMatchObject({
      text: '광고처럼 읽히는 문장을 쓰지 않기',
      scope: ProtoGuidelineScope.TEMPLATES,
      templateIds: ['template-review'],
      fromCandidateId: 'candidate-repeated',
    })
  })

  // A7: a candidate longer than the guideline bound opens for editing with the live remaining
  // count, and approving it unedited is REFUSED by the server rather than truncated.
  it('counts down on a candidate past the guideline bound and relays the refusal', async () => {
    const user = userEvent.setup()
    const long = '가'.repeat(340)
    const creates: FakeGuidelinesOptions['creates'] = []
    renderGuidelines({ candidates: [{ id: 'candidate-long', text: long }], creates })

    const overLong = await candidateRow(long)
    await user.click(overLong.getByRole('button', { name: '승인' }))

    const dialog = within(await screen.findByRole('dialog'))
    // 340 - 300: the count goes negative rather than clamping, so it says how much to cut.
    expect(dialog.getByText('40자 초과')).toBeInTheDocument()
    // The recorded text was never truncated — it is all still here to shorten.
    expect(dialog.getByLabelText('지침')).toHaveValue(long)

    // Saving it unedited reaches no create at all: the client's own field rule stops it, and the
    // server's bound is the one that would refuse it if this check were bypassed.
    await user.click(dialog.getByRole('button', { name: '지침으로 저장' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(creates).toHaveLength(0)

    // Shortened, it saves — the correction was kept rather than lost at recording time.
    await user.clear(dialog.getByLabelText('지침'))
    await user.type(dialog.getByLabelText('지침'), '문단을 짧게')
    await user.click(dialog.getByRole('button', { name: '지침으로 저장' }))
    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toMatchObject({ text: '문단을 짧게', fromCandidateId: 'candidate-long' })
  })

  // A8: the account cap refusal keeps the candidate pending, and the dialog stays open with the
  // draft so nothing typed is lost.
  it('keeps the candidate pending when the account guideline cap refuses the approval', async () => {
    const user = userEvent.setup()
    renderGuidelines({ candidates: CANDIDATES, createAtCap: true })

    const once = await candidateRow('존댓말로 써줘')
    await user.click(once.getByRole('button', { name: '승인' }))

    const dialog = within(await screen.findByRole('dialog'))
    await user.click(dialog.getByRole('button', { name: '지침으로 저장' }))

    await waitFor(() => expect(dialog.getByText(/100/)).toBeInTheDocument())
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    const list = await section('후보 지침')
    expect(list.getAllByRole('listitem')).toHaveLength(3)
  })

  // A6: 무시 marks the row and removes it from the section, with no confirmation — nothing is
  // destroyed, and the row is kept server-side to stop the instruction being recorded again.
  it('dismisses a candidate without a confirmation dialog', async () => {
    const user = userEvent.setup()
    const dismissals: string[] = []
    renderGuidelines({ candidates: CANDIDATES, dismissals })

    const once = await candidateRow('존댓말로 써줘')
    await user.click(once.getByRole('button', { name: '무시' }))

    await waitFor(() => expect(dismissals).toEqual(['candidate-once']))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('존댓말로 써줘')).not.toBeInTheDocument())
  })

  // A duplicate refusal is not silent: no create can ever succeed with that text, so the dialog
  // says why and re-reads the list — the usual way to reach this is another tab having saved it,
  // which also approved this candidate.
  it('says the rule already exists when the approval is refused as a duplicate', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderGuidelines({ candidates: CANDIDATES, createDuplicates: true }, calls)

    const once = await candidateRow('존댓말로 써줘')
    await user.click(once.getByRole('button', { name: '승인' }))

    const dialog = within(await screen.findByRole('dialog'))
    const before = calls.filter((call) => call === 'ListGuidelineCandidates').length
    await user.click(dialog.getByRole('button', { name: '지침으로 저장' }))

    expect(await screen.findByText(/이미 같은 지침이 있어요/)).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ListGuidelineCandidates').length).toBeGreaterThan(
        before,
      ),
    )
  })

  // The previous attempt's refusal goes with its draft: reopening must not show a "too long"
  // or cap message under text that no longer provoked it.
  it('clears a previous refusal when the approval dialog is reopened', async () => {
    const user = userEvent.setup()
    renderGuidelines({ candidates: CANDIDATES, createAtCap: true })

    const once = await candidateRow('존댓말로 써줘')
    await user.click(once.getByRole('button', { name: '승인' }))
    let dialog = within(await screen.findByRole('dialog'))
    await user.click(dialog.getByRole('button', { name: '지침으로 저장' }))
    await waitFor(() => expect(dialog.getByText(/100/)).toBeInTheDocument())
    await user.click(dialog.getByRole('button', { name: '취소' }))

    await user.click((await candidateRow('존댓말로 써줘')).getByRole('button', { name: '승인' }))
    dialog = within(await screen.findByRole('dialog'))
    expect(dialog.queryByText(/100/)).not.toBeInTheDocument()
  })

  // A9: the full queue is the one thing an empty result cannot say.
  it('says the queue is full even with no candidates listed', async () => {
    renderGuidelines({ candidates: [], candidateQueueFull: true })

    const list = await section('후보 지침')
    expect(list.getByText(/후보가 가득 차서/)).toBeInTheDocument()
    expect(list.queryAllByRole('listitem')).toHaveLength(0)
  })

  // Nothing waiting and room to record is the ordinary state: no section, no words about it.
  it('renders no section when nothing is waiting', async () => {
    renderGuidelines({ candidates: [] })
    await screen.findByRole('heading', { level: 1, name: '지침' })
    expect(screen.queryByRole('region', { name: '후보 지침' })).not.toBeInTheDocument()
  })

  // The saved list above owns the page's error state; the candidates are an addition to this
  // screen, not its subject, so their failure adds no second error region.
  it('renders no section when the candidate list cannot be read', async () => {
    renderGuidelines({ candidateListFails: true })
    expect(await screen.findByRole('region', { name: '저장된 지침' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '후보 지침' })).not.toBeInTheDocument()
  })

  // A candidate arrives from a revision the user ran in another tab, so a cached empty list is
  // the wrong answer to "what is waiting for me".
  it('re-reads the candidate list on mount rather than trusting a fresh cache entry', async () => {
    const calls: string[] = []
    const { queryClient, transport } = renderGuidelines({ candidates: CANDIDATES }, calls)
    await screen.findByRole('region', { name: '후보 지침' })
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ListGuidelineCandidates')).toHaveLength(1),
    )
    expect(queryClient.getQueryData(['guideline-candidates', transport, USER.id])).toBeDefined()
  })
})
