import { afterEach, describe, expect, it } from 'vitest'
import { chooseOption } from '@/test/listbox'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { renderAppAt, type RenderAppOptions } from '@/test/app'
import type { FakeVoiceOptions, FakeVoiceRow } from '@/test/voice'

const USER = { id: 'alice' }

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

const VOICES: FakeVoiceRow[] = [
  { id: 'voice-review', name: '리뷰' },
  { id: 'voice-default', name: '기본 말투', isDefault: true },
  { id: 'voice-old', name: '옛 말투', deleted: true },
]

const ANALYZE_MODEL = { providerId: 'stub', modelId: 'analyze' }

/** An account that has chosen an analyze model, which is what a described create needs. */
const WITH_ANALYZE_MODEL: RenderAppOptions['providers'] = {
  models: [{ ...ANALYZE_MODEL, label: 'Analyze' }],
  selections: [{ stage: Stage.ANALYZE, ...ANALYZE_MODEL }],
}

function renderDirectory(
  voice: FakeVoiceOptions = {},
  calls: string[] = [],
  providers: RenderAppOptions['providers'] = WITH_ANALYZE_MODEL,
) {
  return renderAppAt('/voices', {
    user: USER,
    calls,
    providers,
    voice: { voices: VOICES, ...voice },
  })
}

/** A section exists only once the directory has answered, so every lookup waits for it. */
const section = async (name: string) => within(await screen.findByRole('region', { name }))
const deletedGroup = async () =>
  within((await screen.findByText(/삭제된 말투 \d+개/)).closest('details')!)

/** The sheet is mounted only while open, so every creation flow starts by opening it. */
async function openCreateSheet(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '새 말투 만들기' }))
  return within(await screen.findByRole('dialog'))
}

describe('the voice directory', () => {
  // Plan 10 A16 + change 14 A1–A3: a list of rows, tombstones folded away, no form on the page.
  it('lists the active voices as one-target rows and folds the tombstones away', async () => {
    const calls: string[] = []
    renderDirectory({}, calls)

    expect(await screen.findByRole('heading', { level: 1, name: '말투' })).toBeInTheDocument()
    const active = (await section('사용 중')).getAllByRole('link')
    expect(active.map((link) => link.textContent)).toEqual(['기본 말투', '리뷰'])
    expect(active[0]).toHaveAttribute('href', '/voices/voice-default')
    // The row IS the link, so the name carries no underline of its own.
    expect(active[0]).not.toHaveClass('underline')
    // Nothing interactive is nested inside the anchor that covers the row.
    expect(within(active[0]!).queryByRole('button')).not.toBeInTheDocument()
    expect((await section('사용 중')).getByText('기본')).toBeInTheDocument()

    // No creation form on the page — one docked action opens it instead.
    expect(screen.queryByLabelText('말투 이름')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '새 말투 만들기' })).toBeInTheDocument()

    const deleted = await deletedGroup()
    expect(deleted.getByRole('link', { name: '옛 말투' })).toBeInTheDocument()
    expect(deleted.getByText('삭제됨')).toBeInTheDocument()
    expect(screen.getByText('삭제된 말투 1개').closest('details')).not.toHaveAttribute('open')

    // Looking at the directory changes nothing and asks no model anything ([I5]).
    expect(
      calls.filter(
        (call) => !['GetMe', 'ListVoices', 'ListModels', 'GetSelections'].includes(call),
      ),
    ).toEqual([])
  })

  it('renders no tombstone group at all when nothing is deleted', async () => {
    renderDirectory({ voices: VOICES.filter((row) => !row.deleted) })

    await screen.findByRole('heading', { level: 1, name: '말투' })
    expect(screen.queryByText(/삭제된 말투/)).not.toBeInTheDocument()
  })

  it('offers no rename on the directory — the name is edited on the voice', async () => {
    renderDirectory()

    await screen.findByRole('heading', { level: 1, name: '말투' })
    expect(screen.queryByRole('button', { name: '리뷰 이름 바꾸기' })).not.toBeInTheDocument()
  })

  // Change 14 A6: the sheet creates exactly as before when no description is given.
  it('creates a voice from the sheet, starts no job, and lands on the new voice', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const creates: NonNullable<FakeVoiceOptions['creates']> = []
    const { router } = renderDirectory({ creates }, calls)

    const sheet = await openCreateSheet(user)
    await user.type(sheet.getByLabelText('말투 이름'), '  제품 리뷰  ')
    await user.click(sheet.getByRole('button', { name: '말투 만들기' }))

    await waitFor(() => expect(calls).toContain('CreateVoice'))
    expect(creates).toEqual([
      { name: '제품 리뷰', sourceLanguage: 'ko', description: '', analyzeModel: '' },
    ])
    await waitFor(() => expect(router.state.location.pathname).toBe('/voices/voice-4'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  // Change 14 A7: a described create carries the description and the analyze ref, and the run
  // it starts is visible on the voice it lands on.
  it('sends the description with the analyze model and shows the seeding run', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeVoiceOptions['creates']> = []
    const { router } = renderDirectory({ creates, createJobId: 'seed-job' }, [], WITH_ANALYZE_MODEL)

    const sheet = await openCreateSheet(user)
    await user.type(sheet.getByLabelText('말투 이름'), '요리')
    await user.type(
      sheet.getByLabelText('말투 설명 (선택)'),
      '단순하고 차분하지 않은, 농담조의 요리 말투',
    )
    await user.click(sheet.getByRole('button', { name: '말투 만들기' }))

    await waitFor(() =>
      expect(creates).toEqual([
        {
          name: '요리',
          sourceLanguage: 'ko',
          description: '단순하고 차분하지 않은, 농담조의 요리 말투',
          analyzeModel: 'stub/analyze',
        },
      ]),
    )
    await waitFor(() => expect(router.state.location.pathname).toBe('/voices/voice-4'))
    expect(await screen.findByRole('heading', { level: 1, name: '요리' })).toBeInTheDocument()
  })

  it('says a description needs an analyze model and still creates without one', async () => {
    const user = userEvent.setup()
    const creates: NonNullable<FakeVoiceOptions['creates']> = []
    renderDirectory({ creates }, [], { models: [], selections: [] })

    const sheet = await openCreateSheet(user)
    await user.type(sheet.getByLabelText('말투 이름'), '요리')
    await user.type(sheet.getByLabelText('말투 설명 (선택)'), '농담조')
    expect(sheet.getByText(/말투 분석에 쓸 AI 모델을 먼저 골라야/)).toBeInTheDocument()
    expect(sheet.getByRole('button', { name: '말투 만들기' })).toBeDisabled()

    await user.clear(sheet.getByLabelText('말투 설명 (선택)'))
    await user.click(sheet.getByRole('button', { name: '말투 만들기' }))
    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]?.description).toBe('')
  })

  it('defaults a create to the current UI locale and sends it explicitly', async () => {
    initializeI18n('en')
    const creates: NonNullable<FakeVoiceOptions['creates']> = []
    const user = userEvent.setup()
    renderDirectory({ creates })

    await user.click(await screen.findByRole('button', { name: 'New voice' }))
    const sheet = within(await screen.findByRole('dialog'))
    const language = sheet.getByRole('combobox', { name: /Sample language/ })
    expect(language).toHaveTextContent('English')
    await user.type(sheet.getByLabelText('Voice name'), 'English samples')
    await user.click(sheet.getByRole('button', { name: 'Create voice' }))

    await waitFor(() =>
      expect(creates).toEqual([
        { name: 'English samples', sourceLanguage: 'en', description: '', analyzeModel: '' },
      ]),
    )
  })

  it('keeps an explicitly selected source language on a reloaded directory', async () => {
    const creates: NonNullable<FakeVoiceOptions['creates']> = []
    const user = userEvent.setup()
    renderDirectory({ creates })

    const sheet = await openCreateSheet(user)
    await chooseOption(user, sheet.getByRole('combobox', { name: /샘플 언어/ }), '영어')
    await user.type(sheet.getByLabelText('말투 이름'), '영어 말투')
    await user.click(sheet.getByRole('button', { name: '말투 만들기' }))
    await waitFor(() => expect(creates[0]?.sourceLanguage).toBe('en'))

    cleanup()
    renderDirectory({
      voices: [...VOICES, { id: 'voice-english', name: '영어 말투', sourceLanguage: 'en' }],
    })
    const active = await section('사용 중')
    const reloaded = active
      .getAllByRole('listitem')
      .find((item) => within(item).queryByRole('link', { name: '영어 말투' }))
    expect(reloaded).toBeDefined()
    expect(within(reloaded!).getByText('영어')).toBeInTheDocument()
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

  // Plan 10 A4 + change 14 A4: one default at a time, switched from the row itself.
  it('moves the default badge when another voice is set as default from its row', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const { router } = renderDirectory({}, calls)

    await screen.findByRole('heading', { level: 1, name: '말투' })
    await user.click((await section('사용 중')).getByRole('button', { name: '기본으로 설정' }))

    await waitFor(() => expect(calls).toContain('SetDefaultVoice'))
    // A control on the row acts; it does not navigate into the row's voice.
    expect(router.state.location.pathname).toBe('/voices')
    await waitFor(async () => {
      const links = (await section('사용 중')).getAllByRole('link')
      expect(links.map((link) => link.textContent)).toEqual(['리뷰', '기본 말투'])
    })
    expect((await section('사용 중')).getAllByText('기본')).toHaveLength(1)
    // The old default is now deletable, the new one is not.
    expect(
      (await section('사용 중')).getByRole('button', { name: '기본 말투 삭제' }),
    ).toBeInTheDocument()
    expect(
      (await section('사용 중')).queryByRole('button', { name: '리뷰 삭제' }),
    ).not.toBeInTheDocument()
  })

  // Plan 10 A5: a soft delete after confirmation; the default offers no delete at all.
  it('soft-deletes after confirmation and keeps the voice in the tombstone group', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderDirectory({}, calls)

    await screen.findByRole('heading', { level: 1, name: '말투' })
    expect(
      (await section('사용 중')).queryByRole('button', { name: '기본 말투 삭제' }),
    ).not.toBeInTheDocument()
    await user.click((await section('사용 중')).getByRole('button', { name: '리뷰 삭제' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('글과 학습 기록은 그대로 남아요')
    await user.click(within(dialog).getByRole('button', { name: '삭제' }))

    await waitFor(() => expect(calls).toContain('DeleteVoice'))
    await waitFor(async () =>
      expect((await deletedGroup()).getByRole('link', { name: '리뷰' })).toBeInTheDocument(),
    )
    expect((await section('사용 중')).queryByRole('link', { name: '리뷰' })).not.toBeInTheDocument()
  })

  it('explains a refused delete instead of erasing anything', async () => {
    const user = userEvent.setup()
    renderDirectory({ busyVoices: ['voice-review'] })

    await user.click(await screen.findByRole('button', { name: '리뷰 삭제' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '삭제' }),
    )

    expect(await screen.findByRole('alert')).toHaveTextContent('지금은 삭제할 수 없어요')
    expect(screen.getByRole('alert')).toHaveTextContent('이 말투에서 작업이 진행 중이에요.')
    expect((await section('사용 중')).getByRole('link', { name: '리뷰' })).toBeInTheDocument()
  })

  // Plan 10 A6: restore re-lists the voice without touching the default.
  it('restores a deleted voice into the active list without changing the default', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderDirectory({}, calls)

    await screen.findByRole('heading', { level: 1, name: '말투' })
    await user.click((await deletedGroup()).getByRole('button', { name: '복원' }))

    await waitFor(() => expect(calls).toContain('RestoreVoice'))
    await waitFor(async () =>
      expect(
        (await section('사용 중')).getAllByRole('link').map((link) => link.textContent),
      ).toEqual(['기본 말투', '리뷰', '옛 말투']),
    )
    expect(screen.queryByText(/삭제된 말투/)).not.toBeInTheDocument()
    expect((await section('사용 중')).getAllByText('기본')).toHaveLength(1)
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
