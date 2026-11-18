import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoMemoryKind } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeMemoriesOptions, FakeMemoryRow } from '@/test/memories'

const USER = { id: 'alice' }

/** In the SERVER's order — most recently used first — so the assertions below prove the screen
 *  renders that order rather than one of its own. */
const MEMORIES: FakeMemoryRow[] = [
  {
    id: 'memory-place',
    text: '연남동에 자주 간다',
    kind: ProtoMemoryKind.PLACE,
    tags: ['연남동', '산책'],
    sourcePostSlugs: ['post-1'],
  },
  { id: 'memory-preference', text: '매운 음식을 못 먹는다', kind: ProtoMemoryKind.PREFERENCE },
]

function renderMemories(memories: FakeMemoriesOptions = {}, calls: string[] = []) {
  return renderAppAt('/memories', {
    user: USER,
    calls,
    memories: { memories: MEMORIES, ...memories },
  })
}

const ALWAYS = /^늘 떠올리는 기억/
const KEYWORD = /^단어가 나오면 꺼내는 기억/

/** The sheet is mounted only while open, so every creation flow starts by opening it — from the
 *  half the new fact belongs to (MEM-6). */
async function openCreateSheet(user: ReturnType<typeof userEvent.setup>, add = '늘 떠올릴 기억') {
  await user.click(await screen.findByRole('button', { name: add }))
  return within(await screen.findByRole('dialog'))
}

/** One row, by its text, in whichever half holds it. Every row carries the same pencils, so a
 *  query has to be scoped to a row to mean anything. */
async function row(text: string) {
  await screen.findByText(text)
  const found = within(screen.getByRole('main'))
    .getAllByRole('listitem')
    .find((item) => within(item).queryByText(text) !== null)
  if (!found) throw new Error(`no row with text ${text}`)
  return within(found)
}

describe('the memory directory', () => {
  // MEM-23/MEM-24: the screen reads and edits authored facts and nothing else.
  it('splits the memories by when they reach a post, and asks no model anything', async () => {
    const calls: string[] = []
    renderMemories({}, calls)

    // MEM-6: 취향 · 설정 are candidates for every post; 장소 · 인물 · 이력 only on a shared word.
    const always = within(await screen.findByRole('region', { name: ALWAYS }))
    const keyword = within(await screen.findByRole('region', { name: KEYWORD }))
    expect(always.getByText('매운 음식을 못 먹는다')).toBeInTheDocument()
    expect(always.queryByText('연남동에 자주 간다')).not.toBeInTheDocument()
    expect(keyword.getByText('연남동에 자주 간다')).toBeInTheDocument()

    // A word-recalled memory leads with the words that recall it, and says where it came from.
    const place = await row('연남동에 자주 간다')
    expect(place.getByText('이 단어가 나오면')).toBeInTheDocument()
    expect(place.getByText('연남동')).toBeInTheDocument()
    expect(place.getByText('산책')).toBeInTheDocument()
    expect(place.getByText('장소')).toBeInTheDocument()
    expect(place.getByText('글 1편에서 저장')).toBeInTheDocument()
    expect((await row('매운 음식을 못 먹는다')).getByText('직접 적음')).toBeInTheDocument()

    // Reading the directory starts no job and calls no model: a memory is authored text.
    await waitFor(() => expect(calls).toContain('ListMemories'))
    expect(calls.filter((call) => call.startsWith('Start'))).toEqual([])
  })

  it('says a word-recalled memory with no words is never recalled', async () => {
    renderMemories({
      memories: [{ id: 'memory-lonely', text: '단골 빵집이 있다', kind: ProtoMemoryKind.PLACE }],
    })
    expect(
      (await row('단골 빵집이 있다')).getByText('단어가 없어 이 기억은 꺼내지 않아요'),
    ).toBeInTheDocument()
  })

  it('shows both halves, each with its own add, when the account has no memories', async () => {
    renderMemories({ memories: [] })
    expect(await screen.findAllByText('아직 없어요')).toHaveLength(2)
    // A fact the author knows on day one does not require generating a post first (MEM-25).
    expect(screen.getByRole('button', { name: '늘 떠올릴 기억' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '단어로 꺼낼 기억' })).toBeInTheDocument()
  })

  it('opens the shared sheet on the kind of the half it was added from', async () => {
    const user = userEvent.setup()
    renderMemories({ memories: [] })
    const sheet = await openCreateSheet(user, '단어로 꺼낼 기억')
    expect(sheet.getByRole('combobox', { name: /^종류/ })).toHaveTextContent('장소')
  })

  it('renders the failure with a retry rather than an empty directory', async () => {
    renderMemories({ listFails: true })
    expect(await screen.findByRole('alert')).toHaveTextContent('기억을 불러오지 못했어요.')
    expect(screen.queryByRole('region', { name: ALWAYS })).not.toBeInTheDocument()
  })

  // MEM-30: one short row — the text with one pencil and one trash, over one badge line.
  it('keeps a row to its text, one 수정 and one 삭제, and a badge line', async () => {
    renderMemories()
    const place = await row('연남동에 자주 간다')
    expect(place.getAllByRole('button')).toHaveLength(2)
    expect(place.getByRole('button', { name: '기억 수정' })).toBeInTheDocument()
    expect(place.getByRole('button', { name: '이 기억 삭제' })).toBeInTheDocument()
    expect(place.queryByRole('textbox')).not.toBeInTheDocument()
  })

  // One 수정 opens the text, the kind and the tags together, and one 저장 sends only what changed.
  it('edits the text, the kind and the tags in one form and sends them in one request', async () => {
    const user = userEvent.setup()
    const updates: FakeMemoriesOptions['updates'] = []
    renderMemories({ updates })

    const place = await row('연남동에 자주 간다')
    await user.click(place.getByRole('button', { name: '기억 수정' }))
    const field = place.getByLabelText('기억')
    await user.clear(field)
    await user.type(field, '연남동에서 산책한다')
    await user.click(place.getByRole('combobox', { name: /^종류/ }))
    await user.click(await screen.findByRole('option', { name: '취향' }))
    const tags = place.getByLabelText('태그')
    await user.clear(tags)
    await user.type(tags, '연남동, 연남동, 저녁')
    await user.click(place.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toEqual({
      id: 'memory-place',
      text: '연남동에서 산책한다',
      kind: ProtoMemoryKind.PREFERENCE,
      // Duplicates collapse before they are sent, exactly as the server collapses them.
      tags: ['연남동', '저녁'],
    })
    const edited = await row('연남동에서 산책한다')
    expect(edited.getByText('취향')).toBeInTheDocument()
    expect(edited.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('sends only the parts that changed, and nothing when nothing did', async () => {
    const user = userEvent.setup()
    const updates: FakeMemoriesOptions['updates'] = []
    renderMemories({ updates })

    const preference = await row('매운 음식을 못 먹는다')
    await user.click(preference.getByRole('button', { name: '기억 수정' }))
    await user.click(preference.getByRole('button', { name: '저장' }))
    expect(preference.queryByRole('textbox')).not.toBeInTheDocument()
    expect(updates).toEqual([])

    await user.click(preference.getByRole('button', { name: '기억 수정' }))
    await user.type(preference.getByLabelText('태그'), '음식')
    await user.click(preference.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toEqual({
      id: 'memory-preference',
      text: undefined,
      kind: undefined,
      tags: ['음식'],
    })
  })

  it('keeps every typed value on a refused save, and 취소 drops the draft', async () => {
    const user = userEvent.setup()
    const updates: FakeMemoriesOptions['updates'] = []
    renderMemories({ updates })

    const place = await row('연남동에 자주 간다')
    await user.click(place.getByRole('button', { name: '기억 수정' }))
    const field = place.getByLabelText('기억')
    await user.clear(field)
    // Another memory already holds this text, so the server refuses it.
    await user.type(field, '매운 음식을 못 먹는다')
    await user.type(place.getByLabelText('태그'), ', 저녁')
    await user.click(place.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(await place.findByText('이미 같은 기억이 있어요.')).toBeInTheDocument()
    expect(place.getByLabelText('기억')).toHaveValue('매운 음식을 못 먹는다')
    expect(place.getByLabelText('태그')).toHaveValue('연남동, 산책, 저녁')
    expect(place.getByLabelText('기억')).toHaveAttribute('aria-invalid', 'true')

    await user.click(place.getByRole('button', { name: '취소' }))
    expect(place.queryByRole('textbox')).not.toBeInTheDocument()
    expect(await row('연남동에 자주 간다')).toBeTruthy()
    expect(updates).toHaveLength(1)
  })

  it('creates a memory from the sheet and re-reads the list', async () => {
    const user = userEvent.setup()
    const creates: FakeMemoriesOptions['creates'] = []
    renderMemories({ creates })
    await screen.findByRole('region', { name: ALWAYS })

    const sheet = await openCreateSheet(user)
    await user.type(sheet.getByLabelText('기억할 사실'), '성산 일출봉을 좋아한다')
    await user.type(sheet.getByLabelText('태그'), '제주, 성산')
    await user.click(sheet.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toEqual({
      text: '성산 일출봉을 좋아한다',
      kind: ProtoMemoryKind.PREFERENCE,
      tags: ['제주', '성산'],
      // Written by hand: no post is named, so the memory has no source link (MEM-25).
      sourcePostSlug: '',
    })
    expect(await screen.findByText('성산 일출봉을 좋아한다')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  // MEM-11: at the cap nothing is saved and nothing is evicted. The client mirrors no cap — it
  // renders the server's refusal, with the number the server named — and the sheet stays open.
  it('renders the account cap refusal inside the still-open sheet', async () => {
    const user = userEvent.setup()
    renderMemories({ createAtCap: true })
    await screen.findByRole('region', { name: ALWAYS })

    const sheet = await openCreateSheet(user)
    await user.type(sheet.getByLabelText('기억할 사실'), '또 하나의 사실')
    await user.click(sheet.getByRole('button', { name: '저장' }))

    expect(await sheet.findByText(/300개까지 저장할 수 있어요/)).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('deletes a memory after a confirmation that states there is no undo', async () => {
    const user = userEvent.setup()
    const deletions: string[] = []
    renderMemories({ deletions })

    const place = await row('연남동에 자주 간다')
    await user.click(place.getByRole('button', { name: '이 기억 삭제' }))
    const dialog = within(await screen.findByRole('dialog', { name: '기억을 삭제할까요?' }))
    expect(dialog.getByText(/되돌릴 수 없어요/)).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '삭제' }))

    await waitFor(() => expect(deletions).toEqual(['memory-place']))
    await waitFor(() => expect(screen.queryByText('연남동에 자주 간다')).not.toBeInTheDocument())
  })

  // The form replaces the row's controls, so closing it removes the focused 취소 or 저장 from the
  // document. A keyboard user is put back on the pencil they opened it from, not on <body>.
  it("returns focus to 수정 when the row's form closes from the keyboard", async () => {
    const user = userEvent.setup()
    renderMemories()

    const place = await row('연남동에 자주 간다')
    place.getByRole('button', { name: '기억 수정' }).focus()
    await user.keyboard('{Enter}')
    place.getByRole('button', { name: '취소' }).focus()
    await user.keyboard('{Enter}')
    expect(place.queryByRole('textbox')).not.toBeInTheDocument()
    expect(place.getByRole('button', { name: '기억 수정' })).toHaveFocus()

    // 저장 with nothing changed closes the form the same way.
    await user.keyboard('{Enter}')
    place.getByRole('button', { name: '저장' }).focus()
    await user.keyboard('{Enter}')
    expect(place.queryByRole('textbox')).not.toBeInTheDocument()
    expect(place.getByRole('button', { name: '기억 수정' })).toHaveFocus()
  })

  /** Deletes the row holding `text` from the keyboard: Enter on its trash, Enter on 삭제. */
  async function deleteByKeyboard(user: ReturnType<typeof userEvent.setup>, text: string) {
    ;(await row(text)).getByRole('button', { name: '이 기억 삭제' }).focus()
    await user.keyboard('{Enter}')
    const dialog = within(await screen.findByRole('dialog', { name: '기억을 삭제할까요?' }))
    dialog.getByRole('button', { name: '삭제' }).focus()
    await user.keyboard('{Enter}')
    await waitFor(() => expect(screen.queryByText(text)).not.toBeInTheDocument())
  }

  // The confirmation hands focus back to the trash button, and the re-read list then removes the
  // row that holds it. Focus goes to the row that took the deleted one's place in the same half,
  // or to that half's add when its last row went.
  it('keeps keyboard focus in the half a memory was deleted from', async () => {
    const user = userEvent.setup()
    const deletions: string[] = []
    renderMemories({
      deletions,
      memories: [
        ...MEMORIES,
        { id: 'memory-person', text: '동생과 자주 간다', kind: ProtoMemoryKind.PERSON },
      ],
    })

    await deleteByKeyboard(user, '연남동에 자주 간다')
    const next = (await row('동생과 자주 간다')).getByRole('button', { name: '이 기억 삭제' })
    await waitFor(() => expect(next).toHaveFocus())

    await deleteByKeyboard(user, '동생과 자주 간다')
    expect(deletions).toEqual(['memory-place', 'memory-person'])
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '단어로 꺼낼 기억' })).toHaveFocus(),
    )
  })

  it("moves focus to the half's add when the last memory is deleted", async () => {
    const user = userEvent.setup()
    const deletions: string[] = []
    renderMemories({ deletions, memories: [MEMORIES[1]] })

    await deleteByKeyboard(user, '매운 음식을 못 먹는다')
    expect(deletions).toEqual(['memory-preference'])
    expect(await screen.findAllByText('아직 없어요')).toHaveLength(2)
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '늘 떠올릴 기억' })).toHaveFocus(),
    )
  })
})
