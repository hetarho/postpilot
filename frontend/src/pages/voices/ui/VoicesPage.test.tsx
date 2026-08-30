import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeVoiceOptions, FakeVoiceRow } from '@/test/voice'

const USER = { id: 'alice' }

const VOICES: FakeVoiceRow[] = [
  { id: 'voice-review', name: '리뷰' },
  { id: 'voice-default', name: '기본 말투', isDefault: true },
  { id: 'voice-old', name: '옛 말투', deleted: true },
]

function renderDirectory(voice: FakeVoiceOptions = {}, calls: string[] = []) {
  return renderAppAt('/voices', { user: USER, calls, voice: { voices: VOICES, ...voice } })
}

/** A section exists only once the directory has answered, so every lookup waits for it. */
const section = async (name: string) => within(await screen.findByRole('region', { name }))

describe('the voice directory', () => {
  // Plan 10 A16: active voices first (the default leading), the tombstones apart.
  it('lists active voices first, then the deleted ones, and links each to its profile', async () => {
    const calls: string[] = []
    renderDirectory({}, calls)

    expect(await screen.findByRole('heading', { level: 1, name: '말투' })).toBeInTheDocument()
    const active = (await section('사용 중')).getAllByRole('link')
    expect(active.map((link) => link.textContent)).toEqual(['기본 말투', '리뷰'])
    expect(active[0]).toHaveAttribute('href', '/voices/voice-default')
    expect((await section('사용 중')).getByText('기본')).toBeInTheDocument()

    const deleted = (await section('삭제된 말투')).getAllByRole('link')
    expect(deleted.map((link) => link.textContent)).toEqual(['옛 말투'])
    expect((await section('삭제된 말투')).getByText('삭제됨')).toBeInTheDocument()

    // Looking at the directory changes nothing and asks no model anything ([I5]).
    expect(calls.filter((call) => call !== 'GetMe' && call !== 'ListVoices')).toEqual([])
  })

  it('creates a voice from the form and shows it without a reload', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderDirectory({}, calls)

    const name = await screen.findByLabelText('새 말투 이름')
    await user.type(name, '  제품 리뷰  ')
    await user.click(screen.getByRole('button', { name: '말투 만들기' }))

    await waitFor(() => expect(calls).toContain('CreateVoice'))
    expect(
      await (await section('사용 중')).findByRole('link', { name: '제품 리뷰' }),
    ).toBeInTheDocument()
    expect(name).toHaveValue('')
  })

  it('reports a duplicate name under the field and keeps it', async () => {
    const user = userEvent.setup()
    renderDirectory()

    const name = await screen.findByLabelText('새 말투 이름')
    await user.type(name, '리뷰')
    await user.click(screen.getByRole('button', { name: '말투 만들기' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('같은 이름의 말투가 이미 있어요.')
    expect(name).toHaveValue('리뷰')
  })

  // Plan 10 A3: rename changes the display name only.
  it('renames in place', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderDirectory({}, calls)

    await user.click(await screen.findByRole('button', { name: '리뷰 이름 바꾸기' }))
    const field = screen.getByLabelText('말투 이름')
    expect(field).toHaveValue('리뷰')
    await user.clear(field)
    await user.type(field, '제품 리뷰')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(calls).toContain('RenameVoice'))
    expect(
      await (await section('사용 중')).findByRole('link', { name: '제품 리뷰' }),
    ).toBeInTheDocument()
    expect(screen.queryByLabelText('말투 이름')).not.toBeInTheDocument()
  })

  // Plan 10 A4: one default at a time, switched atomically.
  it('moves the default badge when another voice is set as default', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderDirectory({}, calls)

    await screen.findByRole('heading', { level: 1, name: '말투' })
    await user.click((await section('사용 중')).getByRole('button', { name: '기본으로 설정' }))

    await waitFor(() => expect(calls).toContain('SetDefaultVoice'))
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
  it('soft-deletes after confirmation and keeps the voice in the deleted section', async () => {
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
    expect(
      await (await section('삭제된 말투')).findByRole('link', { name: '리뷰' }),
    ).toBeInTheDocument()
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
    expect((await section('사용 중')).getByRole('link', { name: '리뷰' })).toBeInTheDocument()
  })

  // Plan 10 A6: restore re-lists the voice without touching the default.
  it('restores a deleted voice into the active list without changing the default', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderDirectory({}, calls)

    await screen.findByRole('heading', { level: 1, name: '말투' })
    await user.click((await section('삭제된 말투')).getByRole('button', { name: '복원' }))

    await waitFor(() => expect(calls).toContain('RestoreVoice'))
    await waitFor(async () =>
      expect(
        (await section('사용 중')).getAllByRole('link').map((link) => link.textContent),
      ).toEqual(['기본 말투', '리뷰', '옛 말투']),
    )
    expect(screen.queryByRole('region', { name: '삭제된 말투' })).not.toBeInTheDocument()
    expect((await section('사용 중')).getAllByText('기본')).toHaveLength(1)
  })

  it('blocks a restore whose name is taken until the tombstone is renamed', async () => {
    const user = userEvent.setup()
    renderDirectory({
      voices: [
        { id: 'voice-default', name: '기본 말투', isDefault: true },
        { id: 'voice-new', name: '리뷰' },
        { id: 'voice-old', name: '리뷰', deleted: true },
      ],
    })

    await screen.findByRole('heading', { level: 1, name: '말투' })
    await user.click((await section('삭제된 말투')).getByRole('button', { name: '복원' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('이름을 바꾼 뒤 복원해 주세요')

    await user.click(
      (await section('삭제된 말투')).getByRole('button', { name: '리뷰 이름 바꾸기' }),
    )
    const field = screen.getByLabelText('말투 이름')
    await user.clear(field)
    await user.type(field, '옛 리뷰')
    await user.click(screen.getByRole('button', { name: '저장' }))
    await (await section('삭제된 말투')).findByRole('link', { name: '옛 리뷰' })

    await user.click((await section('삭제된 말투')).getByRole('button', { name: '복원' }))
    expect(
      await (await section('사용 중')).findByRole('link', { name: '옛 리뷰' }),
    ).toBeInTheDocument()
  })

  it('says so and offers a retry when the directory cannot be loaded', async () => {
    renderDirectory({ listFails: true })

    expect(await screen.findByRole('alert')).toHaveTextContent('말투 목록을 불러오지 못했어요')
    expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
  })
})
