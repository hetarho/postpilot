import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'
import { createFakeAuthTransport } from '@/test/session'
import type { FakeVoiceOptions, FakeVoiceRow } from '@/test/voice'

const USER = { id: 'alice' }

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

const VOICES: FakeVoiceRow[] = [
  { id: 'voice-review', name: '리뷰', materialCount: 3 },
  { id: 'voice-default', name: '기본 말투', isDefault: true, materialCount: 1 },
  { id: 'voice-new', name: '새 말투', made: false },
  { id: 'voice-old', name: '옛 말투', deleted: true },
]

function renderDirectory(voice: FakeVoiceOptions = {}, calls: string[] = []) {
  return renderAppAt('/voices', { user: USER, calls, voice: { voices: VOICES, ...voice } })
}

/** The active list is the first list on the page, rendered once the directory answered. */
const activeRows = async () => (await screen.findAllByRole('list'))[0]!
const deletedGroup = async () =>
  within((await screen.findByText(/삭제된 말투 \d+개/)).closest('details')!)

/** The sheet is mounted only while open, so every creation flow starts by opening it. */
async function openCreateSheet(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '새 말투 만들기' }))
  return within(await screen.findByRole('dialog'))
}

describe('the voice directory', () => {
  // VOICE-52: 내 글's rows — each voice one link with its name, the 기본 badge and a meta line —
  // tombstones folded away, nothing interactive on a row.
  it('lists the voices as whole-row links with a meta line and folds the tombstones away', async () => {
    const calls: string[] = []
    renderDirectory({}, calls)

    expect(await screen.findByRole('heading', { level: 1, name: '말투' })).toBeInTheDocument()
    const rows = within(await activeRows()).getAllByRole('link')
    expect(rows).toHaveLength(3)
    expect(rows[0]).toHaveAttribute('href', '/voices/voice-default')
    expect(rows[0]).toHaveTextContent('기본 말투')
    expect(rows[0]).toHaveTextContent('기본')
    expect(rows[0]).toHaveTextContent('학습 글 1편 · 2026.08.29 분석')
    expect(rows[1]).toHaveTextContent('리뷰')
    expect(rows[1]).toHaveTextContent('학습 글 3편 · 2026.08.29 분석')
    expect(rows[1]).not.toHaveTextContent('기본')
    expect(rows[2]).toHaveTextContent('새 말투')
    expect(rows[2]).toHaveTextContent('만드는 중 0%')
    // Nothing interactive on a row: no 기본으로 설정, no 삭제, no language chip.
    for (const row of rows) expect(within(row).queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '기본으로 설정' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /삭제$/ })).not.toBeInTheDocument()
    expect(screen.queryByText('한국어')).not.toBeInTheDocument()

    // Both creation methods live in the dock, outside the saved voices.
    expect(screen.queryByLabelText('말투 이름')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '새 말투 만들기' })).toBeInTheDocument()
    expect(
      screen
        .getAllByRole('button', { name: 'AI 말투 추천받기' })
        .find((button) => !button.closest('details')),
    ).toBeInTheDocument()

    const deleted = await deletedGroup()
    expect(deleted.getByRole('link', { name: '옛 말투' })).toBeInTheDocument()
    expect(screen.getByText('삭제된 말투 1개').closest('details')).not.toHaveAttribute('open')

    // Looking at the directory changes nothing and asks no model anything ([I5]).
    expect(
      calls.filter(
        (call) =>
          ![
            'GetMe',
            'GetMyPlan',
            'ListVoices',
            'ListModels',
            'GetSelections',
            'InitializeDefaultSelections',
          ].includes(call),
      ),
    ).toEqual([])
  })

  it('renders no tombstone group at all when nothing is deleted', async () => {
    renderDirectory({ voices: VOICES.filter((row) => !row.deleted) })

    await screen.findByRole('heading', { level: 1, name: '말투' })
    expect(screen.queryByText(/삭제된 말투/)).not.toBeInTheDocument()
  })

  it.each(['close button', 'Escape'] as const)(
    'lets an established account browse AI styles and returns focus on %s without generating work',
    async (dismissal) => {
      const user = userEvent.setup()
      const procedures: string[] = []
      const base = createFakeAuthTransport({
        user: USER,
        existingSetup: true,
        voice: { voices: VOICES },
      })
      const transport = new Proxy(base, {
        get(target, property) {
          if (property !== 'unary') return Reflect.get(target, property)
          return (...args: unknown[]) => {
            procedures.push((args[0] as { name: string }).name)
            return Reflect.apply(target.unary, target, args)
          }
        },
      })
      const { router } = renderAppAt('/voices', { user: USER, transport })
      const disclosure = (await screen.findByText('이전에 만든 말투 이어보기')).closest('details')!
      expect(disclosure).not.toHaveAttribute('open')
      await user.click(within(disclosure).getByText('이전에 만든 말투 이어보기'))
      const trigger = within(disclosure).getByRole('button', { name: 'AI 말투 추천받기' })
      expect(procedures.filter((name) => /WritingVoiceCandidate/.test(name))).toEqual([])
      expect(screen.queryByRole('dialog')).toBeNull()

      await user.click(trigger)
      const dialog = await screen.findByRole('dialog')
      expect(
        await within(dialog).findByText(
          'AI가 서로 다른 느낌의 말투 8개를 만들어요. 예시를 읽고 마음에 드는 스타일 하나를 골라 주세요.',
        ),
      ).toBeVisible()
      await waitFor(() => expect(procedures).toContain('GetLatestWritingVoiceCandidates'))
      expect(procedures.filter((name) => /WritingVoiceCandidate/.test(name))).toEqual([
        'GetLatestWritingVoiceCandidates',
      ])
      expect(procedures.filter((name) => /^(Start|Analyze|CreateVoice|Adopt)/.test(name))).toEqual(
        [],
      )
      expect(router.state.location.pathname).toBe('/voices')

      if (dismissal === 'Escape') await user.keyboard('{Escape}')
      else await user.click(within(dialog).getByRole('button', { name: /^(닫기|돌아가기)$/ }))
      await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
      expect(trigger).toHaveFocus()
      expect(router.state.location.pathname).toBe('/voices')
      expect(procedures.filter((name) => /^(Start|Analyze|CreateVoice|Adopt)/.test(name))).toEqual(
        [],
      )
    },
  )

  // VOICE-4, VOICE-52: an account starts with no voice, and the list says what one is.
  it('says in plain words what a voice is when there is none', async () => {
    renderDirectory({ voices: [] })

    expect(
      await screen.findByText(
        '아직 말투가 없어요. 직접 쓴 글이나 문항 답으로 나만의 말투를 만들어 보세요.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '새 말투 만들기' })).toBeInTheDocument()
  })

  // VOICE-10, VOICE-53: the sheet holds the name and its count alone, starts no job and lands on
  // the new voice.
  it('creates a voice by name alone and lands on the new voice', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const creates: NonNullable<FakeVoiceOptions['creates']> = []
    const { router } = renderDirectory({ creates }, calls)

    const sheet = await openCreateSheet(user)
    expect(sheet.getAllByRole('textbox')).toHaveLength(1)
    expect(sheet.queryByRole('combobox')).not.toBeInTheDocument()
    expect(sheet.getByText('0 / 50자')).toBeInTheDocument()
    await user.type(sheet.getByLabelText('말투 이름'), '  제품 리뷰  ')
    expect(sheet.getByText('5 / 50자')).toBeInTheDocument()
    await user.click(sheet.getByRole('button', { name: '말투 만들기' }))

    await waitFor(() => expect(calls).toContain('CreateVoice'))
    expect(creates).toEqual([{ name: '제품 리뷰' }])
    await waitFor(() => expect(router.state.location.pathname).toBe('/voices/voice-5/materials'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(calls.filter((call) => call.startsWith('Start') || call === 'AddVoiceSample')).toEqual(
      [],
    )
  })

  it('reports a duplicate name inside the sheet and keeps the typed value', async () => {
    const user = userEvent.setup()
    renderDirectory()

    const sheet = await openCreateSheet(user)
    const name = sheet.getByLabelText('말투 이름')
    await user.type(name, '리뷰')
    await user.click(sheet.getByRole('button', { name: '말투 만들기' }))

    expect(await sheet.findByRole('alert')).toHaveTextContent('같은 이름의 말투가 이미 있어요.')
    expect(name).toHaveValue('리뷰')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  // VOICE-14: restore re-lists the voice without touching the default.
  it('restores a deleted voice into the active list without changing the default', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderDirectory({}, calls)

    await screen.findByRole('heading', { level: 1, name: '말투' })
    await user.click((await deletedGroup()).getByRole('button', { name: '복원' }))

    await waitFor(() => expect(calls).toContain('RestoreVoice'))
    await waitFor(async () =>
      expect(within(await activeRows()).getAllByRole('link')).toHaveLength(4),
    )
    expect(screen.queryByText(/삭제된 말투/)).not.toBeInTheDocument()
    expect(within(await activeRows()).getAllByText('기본')).toHaveLength(1)
  })

  it('refuses a restore whose name is taken and says how to resolve it', async () => {
    const user = userEvent.setup()
    renderDirectory({
      voices: [
        { id: 'voice-default', name: '기본 말투', isDefault: true },
        { id: 'voice-new', name: '리뷰' },
        { id: 'voice-old', name: '리뷰', deleted: true },
      ],
    })

    await screen.findByRole('heading', { level: 1, name: '말투' })
    await user.click((await deletedGroup()).getByRole('button', { name: '복원' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('이름을 바꾼 뒤 복원해 주세요')
    expect(screen.getByRole('alert')).toHaveTextContent('같은 이름의 말투가 이미 있어요.')
    // The way out is the voice's own screen, which the row leads to.
    expect((await deletedGroup()).getByRole('link', { name: '리뷰' })).toHaveAttribute(
      'href',
      '/voices/voice-old',
    )
  })

  it('says so and offers a retry when the directory cannot be loaded', async () => {
    renderDirectory({ listFails: true })

    expect(await screen.findByRole('alert')).toHaveTextContent('말투 목록을 불러오지 못했어요')
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
})
