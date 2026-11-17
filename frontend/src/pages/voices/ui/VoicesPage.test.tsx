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

/** The voice new posts are written in, and the other voices, each its own region. */
const writingVoice = async () =>
  within(await screen.findByRole('region', { name: '새 글을 쓰는 말투' }))
const otherVoices = async (count: number) =>
  within(await screen.findByRole('region', { name: `다른 말투 ${count}` }))
const deletedGroup = async () =>
  within((await screen.findByText(/삭제된 말투 \d+개/)).closest('details')!)
const WRITES = /^(Start|Analyze|Create|Adopt|Set|Clear|Delete|Restore|Add)/

/** The three ways to make a voice open from the one 새 말투 in the dock. */
async function chooseWay(user: ReturnType<typeof userEvent.setup>, way: string) {
  await user.click(await screen.findByRole('button', { name: '새 말투 만드는 방법' }))
  await user.click(await screen.findByRole('menuitem', { name: way }))
}

/** The sheet is mounted only while open, so every creation flow starts by opening it. */
async function openCreateSheet(user: ReturnType<typeof userEvent.setup>) {
  await chooseWay(user, '내가 쓴 글로 학습시키기')
  return within(await screen.findByRole('dialog'))
}

describe('the voice directory', () => {
  // The screen opens on the ONE voice new posts are written in; every other voice is a row whose
  // action is what that voice needs; tombstones are folded away.
  it('opens on the writing voice, then the others with the one action each needs', async () => {
    const calls: string[] = []
    renderDirectory({}, calls)

    expect(await screen.findByRole('heading', { level: 1, name: '말투' })).toBeInTheDocument()
    const writing = await writingVoice()
    expect(writing.getByRole('link', { name: '기본 말투' })).toHaveAttribute(
      'href',
      '/voices/voice-default',
    )
    expect(writing.getByText('기본')).toBeInTheDocument()
    expect(writing.getByText('학습 글 1편 · 2026.08.29 분석')).toBeInTheDocument()

    const others = await otherVoices(2)
    const review = others.getByRole('link', { name: '리뷰' })
    expect(review).toHaveAttribute('href', '/voices/voice-review')
    expect(others.getByText('학습 글 3편 · 2026.08.29 분석')).toBeInTheDocument()
    // A made voice offers to become the default; one still being learned offers more writing.
    expect(others.getByRole('button', { name: '기본으로 설정' })).toBeInTheDocument()
    expect(others.getByRole('link', { name: '학습 글 넣기' })).toHaveAttribute(
      'href',
      '/voices/voice-new/materials',
    )
    expect(others.getByText('학습 0%')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /삭제$/ })).not.toBeInTheDocument()

    // 새 말투 is the one creation in the dock; its ways open from it.
    expect(screen.queryByLabelText('말투 이름')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '새 말투 만드는 방법' })).toBeInTheDocument()

    const deleted = await deletedGroup()
    expect(deleted.getByRole('link', { name: '옛 말투' })).toBeInTheDocument()
    expect(screen.getByText('삭제된 말투 1개').closest('details')).not.toHaveAttribute('open')

    // Looking at the directory changes nothing and asks no model anything ([I5]).
    expect(calls.filter((call) => WRITES.test(call))).toEqual([])
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
    const card = await screen.findByRole('region', { name: '새 글을 쓰는 말투' })
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
    const card = await screen.findByRole('region', { name: '새 글을 쓰는 말투' })
    await waitFor(() => expect(card).toHaveTextContent('오늘도 손님이 많았다.'))
    expect(card).not.toHaveTextContent('가상의 예문')
  })

  it('renders no tombstone group at all when nothing is deleted', async () => {
    renderDirectory({ voices: VOICES.filter((row) => !row.deleted) })

    await screen.findByRole('heading', { level: 1, name: '말투' })
    expect(screen.queryByText(/삭제된 말투/)).not.toBeInTheDocument()
  })

  it.each(['close button', 'Escape'] as const)(
    'lets an established account browse AI styles and close them by %s without generating work',
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
      await screen.findByRole('heading', { level: 1, name: '말투' })
      expect(procedures.filter((name) => /WritingVoiceCandidate/.test(name))).toEqual([])
      expect(screen.queryByRole('dialog')).toBeNull()

      await chooseWay(user, '추천 말투에서 고르기')
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
      expect(router.state.location.pathname).toBe('/voices')
      expect(procedures.filter((name) => /^(Start|Analyze|CreateVoice|Adopt)/.test(name))).toEqual(
        [],
      )
    },
  )

  // VOICE-4, VOICE-52: an account starts with no voice, and the list says what one is.
  it('says in plain words what a voice is when there is none', async () => {
    renderDirectory({ voices: [] })

    const writing = await writingVoice()
    expect(writing.getByText('아직 말투가 없어요')).toBeInTheDocument()
    expect(
      writing.getByText('내가 쓴 글을 넣으면 문장 습관을 읽어 내 말투를 만들어요.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '새 말투 만드는 방법' })).toBeInTheDocument()
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
    expect((await otherVoices(3)).getByRole('link', { name: '옛 말투' })).toBeInTheDocument()
    expect(screen.queryByText(/삭제된 말투/)).not.toBeInTheDocument()
    expect((await writingVoice()).getByRole('link', { name: '기본 말투' })).toBeInTheDocument()
    expect(screen.getAllByText('기본')).toHaveLength(1)
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
