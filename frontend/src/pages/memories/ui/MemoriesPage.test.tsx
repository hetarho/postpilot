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

/** The sheet is mounted only while open, so every creation flow starts by opening it (MEM-24). */
async function openCreateSheet(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '새 기억' }))
  return within(await screen.findByRole('dialog'))
}

/** One row of the list, by its text. Every row carries the same pencils, so a query has to be
 *  scoped to a row to mean anything. */
async function row(text: string) {
  const list = within(await screen.findByRole('region', { name: '저장된 기억' }))
  const found = list
    .getAllByRole('listitem')
    .find((item) => within(item).queryByText(text) !== null)
  if (!found) throw new Error(`no row with text ${text}`)
  return within(found)
}

describe('the memory directory', () => {
  // MEM-23/MEM-24: the screen reads and edits authored facts and nothing else.
  it('lists the memories in the server order with their kind and tags, and asks no model anything', async () => {
    const calls: string[] = []
    renderMemories({}, calls)

    const list = within(await screen.findByRole('region', { name: '저장된 기억' }))
    const items = list.getAllByRole('listitem')
    expect(items.map((item) => item.textContent?.includes('연남동에 자주 간다'))).toEqual([
      true,
      false,
    ])
    const place = await row('연남동에 자주 간다')
    expect(place.getByText('장소')).toBeInTheDocument()
    expect(place.getByText('연남동')).toBeInTheDocument()
    expect(place.getByText('산책')).toBeInTheDocument()
    // A memory with no tags says so rather than rendering an empty row of chips.
    expect((await row('매운 음식을 못 먹는다')).getByText('태그 없음')).toBeInTheDocument()

    // Reading the directory starts no job and calls no model: a memory is authored text.
    await waitFor(() => expect(calls).toContain('ListMemories'))
    expect(calls.filter((call) => call.startsWith('Start'))).toEqual([])
  })

  it('says so, and offers no rows, when the account has no memories', async () => {
    renderMemories({ memories: [] })
    expect(await screen.findByRole('heading', { name: '아직 기억이 없어요' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '저장된 기억' })).not.toBeInTheDocument()
    // The one committing action is still there: a fact the author knows on day one does not
    // require generating a post first (MEM-25).
    expect(await screen.findByRole('button', { name: '새 기억' })).toBeInTheDocument()
  })

  it('renders the failure with a retry rather than an empty directory', async () => {
    renderMemories({ listFails: true })
    expect(await screen.findByRole('alert')).toHaveTextContent('기억을 불러오지 못했어요.')
    expect(screen.queryByRole('region', { name: '저장된 기억' })).not.toBeInTheDocument()
  })

  // Read first: the text is prose until the pencil is pressed, and a text edit carries no tags,
  // so two edits from two places cannot overwrite each other.
  it('edits a memory text on request and sends only the text', async () => {
    const user = userEvent.setup()
    const updates: FakeMemoriesOptions['updates'] = []
    renderMemories({ updates })

    const place = await row('연남동에 자주 간다')
    expect(place.queryByRole('textbox')).not.toBeInTheDocument()
    await user.click(place.getByRole('button', { name: '기억 수정' }))
    const field = place.getByRole('textbox')
    await user.clear(field)
    await user.type(field, '연남동에서 산책한다')
    await user.click(place.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toMatchObject({ id: 'memory-place', text: '연남동에서 산책한다' })
    expect(updates[0]?.tags).toBeUndefined()
    expect(updates[0]?.kind).toBeUndefined()
    expect(await screen.findByText('연남동에서 산책한다')).toBeInTheDocument()
  })

  // The kind and the tags are ONE edit: the kind decides whether tags gate the fact at all, so
  // saving one without the other would leave a place fact reachable by nothing (MEM-6).
  it('edits the kind and the tags together and sends no text with them', async () => {
    const user = userEvent.setup()
    const updates: FakeMemoriesOptions['updates'] = []
    renderMemories({ updates })

    const preference = await row('매운 음식을 못 먹는다')
    await user.click(preference.getByRole('button', { name: '종류와 태그 수정' }))
    await user.type(preference.getByLabelText('태그'), '음식, 음식, 저녁')
    await user.click(preference.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]?.text).toBeUndefined()
    expect(updates[0]?.kind).toBe(ProtoMemoryKind.PREFERENCE)
    // Duplicates collapse before they are sent, exactly as the server collapses them.
    expect(updates[0]?.tags).toEqual(['음식', '저녁'])
  })

  it('creates a memory from the sheet and re-reads the list', async () => {
    const user = userEvent.setup()
    const creates: FakeMemoriesOptions['creates'] = []
    renderMemories({ creates })
    await screen.findByRole('region', { name: '저장된 기억' })

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
    await screen.findByRole('region', { name: '저장된 기억' })

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
})
