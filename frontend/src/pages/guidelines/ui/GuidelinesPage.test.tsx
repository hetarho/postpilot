import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoBlogField, ProtoGuidelineKind, ProtoGuidelineScope, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { publishAuthoringDraft } from '@/test/authoring-ui'
import type { FakeAuthoringOptions } from '@/test/authoring'
import type {
  FakeDefaultGuidelineRow,
  FakeGuidelineRow,
  FakeGuidelinesOptions,
} from '@/test/guidelines'
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

/** The sheet is mounted only while open, so every creation flow starts by opening it (GUIDE-20). */
async function openCreateSheet(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '새 지침 직접 쓰기' }))
  const dialog = within(await screen.findByRole('dialog'))
  await dialog.findByLabelText('지침')
  return dialog
}

/** One row of the list, by what its closed line shows (GUIDE-20). Every row carries the same
 *  controls, so a query has to be scoped to a row to mean anything. */
async function row(label: string) {
  const list = await section('지침 목록')
  const found = list
    .getAllByRole('listitem')
    .find((item) => within(item).queryAllByText(label).length > 0)
  if (!found) throw new Error(`no row showing ${label}`)
  return within(found)
}

/** Rows start closed (GUIDE-47): every action on one starts by opening it. */
async function openRow(user: ReturnType<typeof userEvent.setup>, label: string) {
  const found = await row(label)
  await user.click(found.getByRole('button', { expanded: false }))
  return found
}

describe('named owned guideline editing', () => {
  it('lists saved availability and injection order using one batch without fetching individual sessions', async () => {
    const calls: string[] = []
    renderGuidelines({}, calls)
    const list = await section('지침 목록')
    expect(list.getAllByRole('listitem')).toHaveLength(3)
    expect(list.getAllByText('저장되어 사용할 수 있어요')).toHaveLength(3)
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ListAuthoringSummaries')).toHaveLength(1),
    )
    expect(calls).not.toContain('GetLatestAuthoringSession')
    expect(calls).not.toContain('GetAuthoringSession')
    expect(calls).not.toContain('StartAuthoringOperation')
  })
  it('opens a saved named rule first and offers AI/direct methods without editable stock controls', async () => {
    const user = userEvent.setup()
    renderGuidelines()
    const saved = await openRow(user, '없는 사실을 쓰지 않기')
    expect(saved.getByRole('button', { name: /AI로 편집하기$/ })).toBeInTheDocument()
    expect(saved.getByRole('button', { name: /직접 편집하기$/ })).toBeInTheDocument()
    expect(saved.queryByLabelText('지침')).not.toBeInTheDocument()
  })
  it('creates a named global rule only after durable direct review and named publication', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
    renderGuidelines({ guidelines: [], creates })
    const form = await openCreateSheet(user)
    await user.type(form.getByLabelText('제목'), '가격 표기')
    await user.type(form.getByLabelText('지침'), '가격은 원 단위까지 그대로')
    expect(form.getByRole('tab', { name: '전역' })).toHaveAttribute('aria-selected', 'true')
    expect(creates).toHaveLength(0)
    await publishAuthoringDraft(user)
    await waitFor(() => expect(creates).toHaveLength(1))
    expect(await screen.findByText('“가격 표기” 작문 지침을 새로 저장했어요.')).toBeInTheDocument()
    expect(creates[0]).toMatchObject({
      title: '가격 표기',
      text: '가격은 원 단위까지 그대로',
      scope: ProtoGuidelineScope.GLOBAL,
      templateIds: [],
      fields: [],
    })
    expect(await screen.findByRole('button', { name: /가격 표기/ })).toBeInTheDocument()
  })
  it('edits title/text and explicitly changes template scope in one publication', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeGuidelinesOptions['updates']> = []
    renderGuidelines({ updates })
    const saved = await openRow(user, 'CCTV를 언급하지 않기')
    await user.click(saved.getByRole('button', { name: /직접 편집하기$/ }))
    const form = within(await screen.findByRole('dialog'))
    await form.findByLabelText('지침')
    expect(form.getByLabelText('무인가게 리뷰')).toBeChecked()
    await user.type(form.getByLabelText('제목'), '공간 설명')
    await user.clear(form.getByLabelText('지침'))
    await user.type(form.getByLabelText('지침'), '메모의 공간만 설명하기')
    await user.click(form.getByRole('tab', { name: '전역' }))
    await publishAuthoringDraft(user)
    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toMatchObject({
      id: 'guideline-scoped',
      title: '공간 설명',
      text: '메모의 공간만 설명하기',
      scope: { scope: ProtoGuidelineScope.GLOBAL, templateIds: [], fields: [] },
    })
  })
  it('retains incomplete scope and overlong text/title without silently truncating or publishing', async () => {
    const user = userEvent.setup()
    const patches: NonNullable<FakeAuthoringOptions['patches']> = []
    const creates: NonNullable<FakeGuidelinesOptions['creates']> = []
    renderAppAt('/guidelines', {
      user: USER,
      templates: { templates: PURPOSES },
      guidelines: { guidelines: [], creates },
      authoring: { patches },
    })
    const form = await openCreateSheet(user)
    fireEvent.change(form.getByLabelText('지침'), { target: { value: '😀'.repeat(301) } })
    fireEvent.change(form.getByLabelText('제목'), { target: { value: '😀'.repeat(41) } })
    await user.click(form.getByRole('tab', { name: '특정 템플릿' }))
    await user.click(form.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
    await waitFor(() => expect(patches).toHaveLength(1))
    expect(Array.from(patches[0].body)).toHaveLength(301)
    expect(Array.from(patches[0].name)).toHaveLength(41)
    expect(patches[0]).toMatchObject({ scope: 'templates', templateIds: [] })
    expect(creates).toHaveLength(0)
    expect(screen.queryByRole('button', { name: '저장할 내용 확인하기' })).not.toBeInTheDocument()
  })
  it('changes an orphaned scope to fields and clears the previous kind’s link set', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeGuidelinesOptions['updates']> = []
    renderGuidelines({ updates })
    const saved = await openRow(user, '주인 이야기를 쓰지 않기')
    await user.click(saved.getByRole('button', { name: /직접 편집하기$/ }))
    const form = within(await screen.findByRole('dialog'))
    await form.findByLabelText('지침')
    await user.click(form.getByRole('tab', { name: '특정 분야' }))
    await user.click(form.getByLabelText('맛집'))
    await publishAuthoringDraft(user)
    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0].scope).toEqual({
      scope: ProtoGuidelineScope.FIELDS,
      templateIds: [],
      fields: [ProtoBlogField.RESTAURANT],
    })
  })
  it('preserves duplicate refusal and never adds generated rules to the revision candidate queue', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderGuidelines({ createDuplicates: true }, calls)
    const form = await openCreateSheet(user)
    await user.type(form.getByLabelText('지침'), '이미 있는 규칙')
    await publishAuthoringDraft(user)
    expect(await screen.findByText(/이미.*지침/)).toBeInTheDocument()
    expect(calls).not.toContain('StartAuthoringOperation')
    expect(calls).not.toContain('DismissGuidelineCandidate')
  })
  it('explains frozen admitted text before deletion and retries failed directory reads', async () => {
    const user = userEvent.setup()
    const rendered = renderGuidelines()
    const saved = await openRow(user, '없는 사실을 쓰지 않기')
    await user.click(saved.getByRole('button', { name: '지침 삭제' }))
    expect(await screen.findByRole('dialog')).toHaveTextContent(/이미|진행|작업/)
    rendered.unmount()
    renderGuidelines({ listFails: true })
    expect(await screen.findByText('지침 목록을 불러오지 못했어요.')).toBeInTheDocument()
  })
})

const DEFAULTS: FakeDefaultGuidelineRow[] = [
  { key: 'facts', name: '재료에 있는 사실만', text: '메모와 사진 관찰에 없는 사실은 쓰지 마세요.' },
  { key: 'naming', name: '메모의 이름으로', text: '메모에 적힌 이름으로 쓰세요.', enabled: false },
  {
    key: 'natural_korean',
    name: '자연스러운 한국어 문체',
    text: '상투적인 대조를 줄이세요.',
    koreanTargetOnly: true,
  },
]

/** GUIDE-20, GUIDE-43, GUIDE-47, GUIDE-48: the 기본 지침 in use are rows of the one list, first and
 *  in registry order; one leaves it by 적용 안함 and comes back from the 기본 지침 sheet. */
describe('the 기본 지침', () => {
  it('lists the ones in use first, in registry order, in the same list as the owner’s rows', async () => {
    renderGuidelines({ defaults: DEFAULTS })

    const list = await section('지침 목록')
    const items = list.getAllByRole('listitem')
    // naming is out of use, so it is not a row at all.
    expect(items).toHaveLength(2 + GUIDELINES.length)
    expect(within(items[0]).getByRole('button', { expanded: false })).toHaveTextContent(
      '재료에 있는 사실만추천',
    )
    expect(within(items[1]).getByText('자연스러운 한국어 문체')).toBeInTheDocument()
    expect(within(items[2]).getByText('없는 사실을 쓰지 않기')).toBeInTheDocument()
    expect(list.queryByText('메모의 이름으로')).not.toBeInTheDocument()
    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
    // Closed rows carry no text and no action.
    expect(list.queryByText('메모와 사진 관찰에 없는 사실은 쓰지 마세요.')).not.toBeInTheDocument()
    expect(list.queryByRole('button', { name: /적용 안함/ })).not.toBeInTheDocument()
  })

  it('opens rows independently, each showing what it says and its action', async () => {
    const user = userEvent.setup()
    renderGuidelines({ defaults: DEFAULTS })

    const korean = await openRow(user, '자연스러운 한국어 문체')
    expect(korean.getByText('상투적인 대조를 줄이세요.')).toBeInTheDocument()
    expect(korean.getByText('한국어 글에만 적용돼요')).toBeInTheDocument()
    expect(
      korean.getByRole('button', { name: '자연스러운 한국어 문체 적용 안함' }),
    ).toBeInTheDocument()
    const owner = await openRow(user, '없는 사실을 쓰지 않기')
    expect(owner.getByRole('button', { name: /직접 편집하기$/ })).toBeInTheDocument()
    expect(owner.getByRole('button', { name: '지침 삭제' })).toBeInTheDocument()
    // Opening the second left the first open.
    expect(korean.getByText('상투적인 대조를 줄이세요.')).toBeInTheDocument()

    await user.click(korean.getByRole('button', { expanded: true }))
    expect(korean.queryByText('상투적인 대조를 줄이세요.')).not.toBeInTheDocument()
  })

  // The empty state is for a list with nothing in it; 기본 지침 in use are in the list.
  it('shows the list rather than the empty state while a 기본 지침 is in use', async () => {
    renderGuidelines({ defaults: DEFAULTS, guidelines: [] })
    expect(await section('지침 목록')).toBeTruthy()
    expect(screen.queryByRole('region', { name: '아직 적용 중인 지침이 없어요' })).toBeNull()
  })

  it('takes one out of use at once with 적용 안함, and the change survives a refetch', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const defaultSwitches: FakeGuidelinesOptions['defaultSwitches'] = []
    const { queryClient, transport } = renderGuidelines(
      { defaults: DEFAULTS, defaultSwitches },
      calls,
    )

    const facts = await openRow(user, '재료에 있는 사실만')
    await user.click(facts.getByRole('button', { name: '재료에 있는 사실만 적용 안함' }))
    // No dialog: putting it back is one press in the sheet.
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await waitFor(() =>
      expect(defaultSwitches).toEqual([
        { kind: ProtoGuidelineKind.POST, key: 'facts', enabled: false },
      ]),
    )
    await waitFor(async () =>
      expect((await section('지침 목록')).queryByText('재료에 있는 사실만')).toBeNull(),
    )

    const reads = calls.filter((call) => call === 'ListGuidelines').length
    await queryClient.invalidateQueries({ queryKey: ['guidelines', transport, USER.id] })
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ListGuidelines').length).toBeGreaterThan(reads),
    )
    expect((await section('지침 목록')).queryByText('재료에 있는 사실만')).toBeNull()
  })

  it('puts a refused 적용 안함 back and says why above the list', async () => {
    const user = userEvent.setup()
    renderGuidelines({ defaults: DEFAULTS, refuseDefaultSwitch: true })

    const facts = await openRow(user, '재료에 있는 사실만')
    await user.click(facts.getByRole('button', { name: '재료에 있는 사실만 적용 안함' }))
    expect(await screen.findByText('기본 지침을 찾을 수 없어요.')).toBeInTheDocument()
    await waitFor(async () =>
      expect((await section('지침 목록')).getByText('재료에 있는 사실만')).toBeInTheDocument(),
    )
  })

  it('lists every one in the sheet and adds one there without closing it', async () => {
    const user = userEvent.setup()
    const defaultSwitches: FakeGuidelinesOptions['defaultSwitches'] = []
    renderGuidelines({ defaults: DEFAULTS, defaultSwitches })

    await user.click(await screen.findByRole('button', { name: '기본 지침' }))
    const sheet = within(await screen.findByRole('dialog', { name: '기본 지침' }))
    const options = sheet.getAllByRole('listitem')
    expect(options.map((option) => within(option).getByRole('heading').textContent)).toEqual([
      '재료에 있는 사실만',
      '메모의 이름으로',
      '자연스러운 한국어 문체',
    ])
    // Each with its text, so the choice is informed; the ones in use carry no control.
    expect(sheet.getByText('메모에 적힌 이름으로 쓰세요.')).toBeInTheDocument()
    expect(within(options[0]).getByText('적용 중')).toBeInTheDocument()
    expect(within(options[0]).queryByRole('button')).toBeNull()

    await user.click(sheet.getByRole('button', { name: '메모의 이름으로 추가' }))
    await waitFor(() =>
      expect(defaultSwitches).toEqual([
        { kind: ProtoGuidelineKind.POST, key: 'naming', enabled: true },
      ]),
    )
    expect(screen.getByRole('dialog', { name: '기본 지침' })).toBeInTheDocument()
    await waitFor(() => expect(within(options[1]).getByText('적용 중')).toBeInTheDocument())

    await user.click(sheet.getByRole('button', { name: '닫기' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    // In the list at its product position: after facts, before natural_korean.
    const items = (await section('지침 목록')).getAllByRole('listitem')
    expect(within(items[1]).getByText('메모의 이름으로')).toBeInTheDocument()
  })

  it('says why a refused 추가 failed under its row in the sheet', async () => {
    const user = userEvent.setup()
    renderGuidelines({ defaults: DEFAULTS, refuseDefaultSwitch: true })

    await user.click(await screen.findByRole('button', { name: '기본 지침' }))
    const sheet = within(await screen.findByRole('dialog', { name: '기본 지침' }))
    await user.click(sheet.getByRole('button', { name: '메모의 이름으로 추가' }))
    const naming = within(sheet.getAllByRole('listitem')[1])
    expect(await naming.findByText('기본 지침을 찾을 수 없어요.')).toBeInTheDocument()
    expect(naming.getByRole('button', { name: '메모의 이름으로 추가' })).toBeInTheDocument()
  })
})

/** The 후보 section (GUIDE-22). Every row is one instruction a completed revision recorded
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

  const disclosure = async () => (await screen.findByText(/지침 후보 \d+개/)).closest('details')!

  // GUIDE-22: the saved rules are what the screen is for, so the queue is folded away below them
  // and says how much is waiting.
  it('folds the queue away below the saved list with its pending count', async () => {
    renderGuidelines({ candidates: CANDIDATES })

    const details = await disclosure()
    expect(details).not.toHaveAttribute('open')
    const saved = await screen.findByRole('region', { name: '지침 목록' })
    // Below the list, not above it: DOCUMENT_POSITION_FOLLOWING.
    expect(saved.compareDocumentPosition(details) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  // GUIDE-27: everything that passes is saved 전역, in one go.
  it('accepts the whole queue as 전역 guidelines', async () => {
    const user = userEvent.setup()
    const creates: FakeGuidelinesOptions['creates'] = []
    renderGuidelines({ candidates: CANDIDATES, creates })

    await user.click(await screen.findByRole('button', { name: '전부 수락' }))

    await waitFor(() => expect(creates).toHaveLength(3))
    expect(creates.map((call) => call.text)).toEqual([
      '여기 너무 광고 같아',
      '존댓말로 써줘',
      '문단을 짧게',
    ])
    // Each one carries its own row's id and the global scope, which takes no template ids.
    expect(creates.map((call) => call.fromCandidateId)).toEqual([
      'candidate-repeated',
      'candidate-once',
      'candidate-orphan',
    ])
    expect(creates.every((call) => call.scope === ProtoGuidelineScope.GLOBAL)).toBe(true)
    expect(creates.every((call) => call.templateIds.length === 0)).toBe(true)
    expect(creates.every((call) => call.fields.length === 0)).toBe(true)
    // A candidate has no title and the bulk path asks for none (GUIDE-46).
    expect(creates.every((call) => call.title === undefined)).toBe(true)
    expect(
      await screen.findByText('3개를 지침으로 저장했어요. 0개는 그대로 남았어요.'),
    ).toBeInTheDocument()
    const list = await section('지침 목록')
    await waitFor(() => expect(list.getByText('여기 너무 광고 같아')).toBeInTheDocument())
  })

  // GUIDE-27: one refusal must not hold back the rest, and it has to say which row it was.
  it('keeps a refused candidate with its reason and saves the rest', async () => {
    const user = userEvent.setup()
    const creates: FakeGuidelinesOptions['creates'] = []
    renderGuidelines({
      // The middle one is already a saved rule, so the server refuses exactly that create.
      candidates: [
        { id: 'candidate-fresh', text: '가격을 지어내지 않기' },
        { id: 'candidate-dupe', text: '없는 사실을 쓰지 않기' },
        { id: 'candidate-last', text: '문단을 짧게' },
      ],
      creates,
    })

    await user.click(await screen.findByRole('button', { name: '전부 수락' }))

    // The walk did not abort at the refusal: all three were attempted, two saved.
    await waitFor(() => expect(creates).toHaveLength(3))
    expect(
      await screen.findByText('2개를 지침으로 저장했어요. 1개는 그대로 남았어요.'),
    ).toBeInTheDocument()
    const refused = await candidateRow('없는 사실을 쓰지 않기')
    expect(refused.getByText('이미 같은 지침이 있어요.')).toBeInTheDocument()
    // The two that saved left the queue; the refused one is still there to fix by hand.
    const list = await section('후보 지침')
    await waitFor(() => expect(list.getAllByRole('listitem')).toHaveLength(1))
  })

  // GUIDE-27: fifty rows at once have no undo, so this one asks — unlike a single 무시.
  it('asks once before dismissing the whole queue and cancels cleanly', async () => {
    const user = userEvent.setup()
    const dismissals: string[] = []
    renderGuidelines({ candidates: CANDIDATES, dismissals })

    await user.click(await screen.findByRole('button', { name: '전부 거절' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText('후보 3개를 전부 거절할까요?')).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '취소' }))
    expect(dismissals).toEqual([])

    await user.click(screen.getByRole('button', { name: '전부 거절' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '전부 거절' }),
    )

    await waitFor(() => expect(dismissals).toHaveLength(3))
    expect(dismissals).toEqual(['candidate-repeated', 'candidate-once', 'candidate-orphan'])
    expect(await screen.findByText('3개를 거절했어요.')).toBeInTheDocument()
  })

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
    const allowed = [
      'InitializeDefaultSelections',
      'GetMe',
      'GetMyPlan',
      'ListGuidelines',
      'ListGuidelineCandidates',
      'ListAuthoringSummaries',
      'ListTemplates',
    ]
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
    // 전역 is preselected: a rule applies everywhere unless the user narrows it — to templates or
    // to 분야, the shared control's three choices.
    expect(dialog.getByRole('tab', { name: '전역' })).toHaveAttribute('aria-selected', 'true')
    expect(dialog.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
      '전역',
      '특정 템플릿',
      '특정 분야',
    ])
    // A candidate carries no title, so the approval's title field opens empty (GUIDE-46).
    expect(dialog.getByLabelText('제목')).toHaveValue('')
    await user.type(dialog.getByLabelText('제목'), '광고 같은 문장')
    await user.click(dialog.getByRole('button', { name: '지침으로 저장' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toMatchObject({
      title: '광고 같은 문장',
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
    const saved = await section('지침 목록')
    expect(saved.getAllByRole('listitem')).toHaveLength(4)
    // The approval named it, so its closed row shows the title (GUIDE-46).
    expect(saved.getByText('광고 같은 문장')).toBeInTheDocument()
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
    // Nothing to accept or reject in bulk when there is no row to move.
    expect(list.queryByRole('button', { name: '전부 수락' })).not.toBeInTheDocument()
    expect(list.queryByRole('button', { name: '전부 거절' })).not.toBeInTheDocument()
  })

  // GUIDE-22, F3: the closed disclosure's summary says the queue is full, so a full queue is seen
  // without opening it.
  it('says the queue is full in the closed summary', async () => {
    renderGuidelines({ candidates: [], candidateQueueFull: true })

    const summary = await screen.findByText('지침 후보 0개 · 가득 참')
    expect(summary.closest('details')).not.toHaveAttribute('open')
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
    expect(await screen.findByRole('region', { name: '지침 목록' })).toBeInTheDocument()
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

// TMPL-61: the template request's 지침 만들기 lands here with `?new=1`, which opens an empty new
// guideline at once; closing it drops the parameter so a reload does not open it again.
describe('/guidelines?new=1', () => {
  it('opens the AI guideline draft without generating work and drops the parameter on close', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const { router } = renderAppAt('/guidelines?new=1', {
      user: USER,
      calls,
      providers: {
        models: [
          { providerId: 'stub', modelId: 'recommended', label: '추천 AI', stages: [Stage.WRITE] },
        ],
        selections: [{ stage: Stage.WRITE, providerId: 'stub', modelId: 'recommended' }],
      },
    })
    const sheet = await screen.findByRole('dialog', {
      name: '마음에 드는 작문 지침, 함께 만들어요',
    })
    expect(within(sheet).queryByLabelText('지침')).not.toBeInTheDocument()
    expect(await within(sheet).findByRole('button', { name: '8가지 추천받기' })).toBeEnabled()
    await waitFor(() => expect(calls).toContain('ListAuthoringSummaries'))
    expect(calls.filter((call) => /^(Start|Analyze|Create|SaveAuthoring)/.test(call))).toEqual([])

    await user.keyboard('{Escape}')
    await waitFor(() =>
      expect(
        screen.queryByRole('dialog', { name: '마음에 드는 작문 지침, 함께 만들어요' }),
      ).not.toBeInTheDocument(),
    )
    await waitFor(() => expect(router.state.location.search).toEqual({}))
  })
})
