import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'
import { ProtoVoiceOrigin } from '@/shared/api'
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

/** The voice posts are written in, drawn large, and every other active voice as one row. */
const portrait = async () => within(await screen.findByRole('article'))
const otherRows = async () => within(await screen.findByRole('region', { name: '다른 말투' }))
const deletedGroup = async () =>
  within((await screen.findByText(/삭제된 말투 \d+개/)).closest('details')!)

/** The sheet is mounted only while open, so every creation flow starts by opening it — from the
 *  ONE 새 말투 menu in the heading row, whose 내 글로 배우기 is this sheet. */
async function openCreateSheet(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '새 말투' }))
  await user.click(await screen.findByRole('menuitem', { name: /내 글로 배우기/ }))
  return within(await screen.findByRole('dialog'))
}

describe('the voice directory', () => {
  // The writing voice leads, drawn large; every other voice is one row whose made voice takes the
  // writing voice's place in one tap; tombstones fold away.
  it('leads with the writing voice and lists the others with a one-tap default', async () => {
    const calls: string[] = []
    renderDirectory({}, calls)

    expect(await screen.findByRole('heading', { level: 1, name: '말투' })).toBeInTheDocument()
    const current = await portrait()
    expect(current.getByRole('heading', { name: '기본 말투' })).toBeInTheDocument()
    expect(current.getByText('기본')).toBeInTheDocument()
    expect(current.getByText('학습 글 1편 · 2026.08.29 분석')).toBeInTheDocument()
    expect(current.getByRole('link', { name: /자세히 보기/ })).toHaveAttribute(
      'href',
      '/voices/voice-default',
    )

    const others = await otherRows()
    expect(others.getByRole('link', { name: '리뷰' })).toHaveAttribute(
      'href',
      '/voices/voice-review',
    )
    expect(others.getByRole('link', { name: '새 말투' })).toHaveAttribute(
      'href',
      '/voices/voice-new',
    )
    expect(others.getByText('학습 0%')).toBeInTheDocument()
    // Only a made voice can become the default (VOICE-12): one button, on 리뷰's row.
    expect(others.getAllByRole('button', { name: '기본으로 설정' })).toHaveLength(1)

    // The ways to make a voice wait behind the one 새 말투; none sits on the page by itself.
    expect(screen.queryByLabelText('말투 이름')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '새 말투' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'AI 말투 추천받기' })).not.toBeInTheDocument()

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
            'GetVoiceProfile',
            'ListModels',
            'GetSelections',
            'InitializeDefaultSelections',
            // The header draws the 말투 ring 홈 draws, from the same reads 홈 makes.
            'ListTemplates',
            'ListGuidelines',
            'ListMemories',
          ].includes(call),
      ),
    ).toEqual([])
  })

  it('offers the three ways to make a voice from the one 새 말투', async () => {
    const user = userEvent.setup()
    renderDirectory()
    await user.click(await screen.findByRole('button', { name: '새 말투' }))
    expect(await screen.findAllByRole('menuitem')).toHaveLength(3)
    expect(screen.getByRole('menuitem', { name: /내 글로 배우기/ })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: /AI와 대화로 만들기/ })).toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: /이전에 만든 말투 이어보기/ })).toBeInTheDocument()
  })

  // VOICE-69: a style an AI made was read from a fictional sample, and the card quoting that
  // sample says so; a voice learned from the owner's own writing carries no such label.
  it('labels the sample an AI-made style shows as fictional', async () => {
    renderAppAt('/voices', {
      user: USER,
      voice: {
        voices: [{ id: 'voice-ai', name: 'AI 말투', isDefault: true, origin: 'synthetic' }],
        analysis: {
          origin: ProtoVoiceOrigin.SYNTHETIC,
          syntheticSample: '바다가 보이는 창가에서 커피를 마셨어요.',
          counted: {},
          ai: {},
        },
      },
    })
    await screen.findByRole('heading', { level: 1, name: '말투' })
    const card = (await portrait()).getByRole('heading', { name: 'AI 말투' }).closest('article')!
    await waitFor(() => expect(card).toHaveTextContent('바다가 보이는 창가에서 커피를 마셨어요.'))
    expect(card).toHaveTextContent('AI가 만든 가상의 예문')
  })

  it("leaves a sample from the owner's own writing unlabelled", async () => {
    renderAppAt('/voices', {
      user: USER,
      voice: {
        voices: [{ id: 'voice-mine', name: '내 말투', isDefault: true }],
        samples: [{ id: 'sample-1', label: '가게 일기' }],
        analysis: {
          origin: ProtoVoiceOrigin.PERSONAL,
          counted: {},
          ai: {
            examples: [{ field: 1, sentence: '오늘도 손님이 많았다.', materialId: 'sample-1' }],
          },
        },
      },
    })
    await screen.findByRole('heading', { level: 1, name: '말투' })
    const card = (await portrait()).getByRole('heading', { name: '내 말투' }).closest('article')!
    await waitFor(() => expect(card).toHaveTextContent('오늘도 손님이 많았다.'))
    expect(card).not.toHaveTextContent('가상의 예문')
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
      const trigger = await screen.findByRole('button', { name: '새 말투' })
      expect(procedures.filter((name) => /WritingVoiceCandidate/.test(name))).toEqual([])
      expect(screen.queryByRole('dialog')).toBeNull()

      await user.click(trigger)
      await user.click(await screen.findByRole('menuitem', { name: /이전에 만든 말투 이어보기/ }))
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
        '아직 내 말투가 없어요. 글을 보여 주거나 AI와 이야기해서 하나 만들어 보세요.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '새 말투' })).toBeInTheDocument()
  })

  // THEME-67: `?new=1` — the section's 새 말투 — opens the sheet at once, and closing it
  // ignores an old link's channel.
  it('opens the creation sheet from ?new=1 and drops the request on close', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/voices?new=1', {
      user: USER,
      voice: { voices: VOICES },
    })
    const sheet = within(await screen.findByRole('dialog'))
    expect(sheet.getByLabelText('말투 이름')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(router.state.location.search).toEqual({}))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
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
      expect((await otherRows()).getByRole('link', { name: '옛 말투' })).toBeInTheDocument(),
    )
    expect(screen.queryByText(/삭제된 말투/)).not.toBeInTheDocument()
    // The default stays the writing voice: still the one drawn large, with its badge.
    expect((await portrait()).getByRole('heading', { name: '기본 말투' })).toBeInTheDocument()
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
