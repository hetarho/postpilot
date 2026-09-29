import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code } from '@connectrpc/connect'
import { afterEach, expect, it, vi } from 'vitest'
import { connectAppError } from '@/test/app-error'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import type { FakeVoiceRow } from '@/test/voice'
import { reassignmentBlocker } from '../model/reassignment'
import { PostVoiceSelect } from './PostVoiceSelect'

afterEach(cleanup)

const TWO_VOICES: FakeVoiceRow[] = [
  { id: 'voice-default', name: '기본 말투', isDefault: true },
  { id: 'voice-review', name: '리뷰' },
]
const CURRENT = {
  id: 'voice-default',
  name: '기본 말투',
  deleted: false,
  made: true,
  sourceLanguage: 'ko' as const,
}

function renderSelect(
  props: Partial<Parameters<typeof PostVoiceSelect>[0]> = {},
  voices: FakeVoiceRow[] = TWO_VOICES,
) {
  const onSelect = vi.fn<(voiceId: string) => Promise<void>>(async () => {})
  const onCreateVoice = vi.fn()
  const transport = createFakeAuthTransport({ user: { id: 'alice' }, voice: { voices } })
  render(
    <PostVoiceSelect
      ownerId="alice"
      value="voice-default"
      current={CURRENT}
      confirm
      onSelect={onSelect}
      onCreateVoice={onCreateVoice}
      {...props}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { onSelect, onCreateVoice }
}

const picker = () => screen.findByRole('combobox', { name: /말투/ })

/** The directory answers after the first paint: open the list only once the field is live. */
async function openList(user: ReturnType<typeof userEvent.setup>) {
  const field = await picker()
  await waitFor(() => expect(field).toBeEnabled())
  await user.click(field)
  return screen.findByRole('listbox')
}

async function pickReview(user: ReturnType<typeof userEvent.setup>) {
  await user.click(within(await openList(user)).getByRole('option', { name: '리뷰' }))
  return screen.findByRole('dialog', { name: '말투를 바꿀까요?' })
}

// POST-101: 말투 없음, the made voices, each one not yet made (listed, not choosable), and the way
// to make a new one — in that order.
it('lists 말투 없음, the made voices, the unmade ones disabled, then 새 말투 만들기', async () => {
  const user = userEvent.setup()
  renderSelect({}, [
    ...TWO_VOICES,
    { id: 'voice-cafe', name: '가게 소개', made: false },
    { id: 'voice-gone', name: '옛 말투', deleted: true },
  ])

  const options = within(await openList(user)).getAllByRole('option')

  expect(options.map((option) => option.textContent)).toEqual([
    '말투 없음',
    '기본 말투',
    '리뷰',
    '가게 소개 · 만드는 중',
    '새 말투 만들기',
  ])
  expect(options.map((option) => option.getAttribute('aria-disabled') === 'true')).toEqual([
    false,
    false,
    false,
    true,
    false,
  ])
})

it('keeps the selection when a voice not yet made is pressed', async () => {
  const user = userEvent.setup()
  const { onSelect } = renderSelect({}, [
    ...TWO_VOICES,
    { id: 'voice-cafe', name: '가게 소개', made: false },
  ])

  await user.click(
    within(await openList(user)).getByRole('option', { name: '가게 소개 · 만드는 중' }),
  )

  expect(onSelect).not.toHaveBeenCalled()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

// VOICE-53: the option opens the create sheet (the caller's) and is not an assignment.
it('asks for the create sheet from 새 말투 만들기 and leaves the selection alone', async () => {
  const user = userEvent.setup()
  const { onSelect, onCreateVoice } = renderSelect()

  await user.click(within(await openList(user)).getByRole('option', { name: '새 말투 만들기' }))

  expect(onCreateVoice).toHaveBeenCalledOnce()
  expect(onSelect).not.toHaveBeenCalled()
  expect(screen.queryByRole('dialog', { name: '말투를 바꿀까요?' })).not.toBeInTheDocument()
  expect(await picker()).toHaveTextContent('기본 말투')
})

// POST-24: clearing is a reassignment like any other, so an existing post is asked first and the
// editor is handed ''.
it('clears an existing post to 말투 없음 through the confirmation', async () => {
  const user = userEvent.setup()
  const { onSelect } = renderSelect()

  await user.click(within(await openList(user)).getByRole('option', { name: '말투 없음' }))
  const sheet = await screen.findByRole('dialog', { name: '말투를 바꿀까요?' })
  expect(sheet).toHaveTextContent('‘말투 없음’(으)로 바꿉니다')
  await user.click(within(sheet).getByRole('button', { name: '말투 변경' }))

  expect(onSelect).toHaveBeenCalledWith('')
})

it('switches a draft with no post yet to 말투 없음 without asking', async () => {
  const user = userEvent.setup()
  const { onSelect } = renderSelect({ confirm: false, current: undefined })

  await user.click(within(await openList(user)).getByRole('option', { name: '말투 없음' }))

  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(onSelect).toHaveBeenCalledWith('')
})

it('reads 말투 없음 on a post that has none', async () => {
  renderSelect({ value: '', current: undefined })
  expect(await picker()).toHaveTextContent('말투 없음')
})

// POST-25: the post's own deleted voice stays listed, so the field still says what the post is
// written in, but it cannot be chosen again.
it("lists the post's own deleted voice, disabled", async () => {
  const user = userEvent.setup()
  renderSelect({
    value: 'voice-gone',
    current: { id: 'voice-gone', name: '옛 말투', deleted: true, made: true, sourceLanguage: 'ko' },
  })

  expect(await picker()).toHaveTextContent('삭제된 말투 · 옛 말투')
  const option = within(await openList(user)).getByRole('option', { name: '삭제된 말투 · 옛 말투' })
  expect(option).toHaveAttribute('aria-disabled', 'true')
})

it('cancels a reassignment from the sheet without saving anything', async () => {
  const user = userEvent.setup()
  const { onSelect } = renderSelect()

  await user.click(within(await pickReview(user)).getByRole('button', { name: '취소' }))

  expect(screen.queryByRole('dialog', { name: '말투를 바꿀까요?' })).not.toBeInTheDocument()
  expect(await picker()).toHaveTextContent('기본 말투')
  expect(onSelect).not.toHaveBeenCalled()
})

it('blocks reassignment while a job is active and says why', async () => {
  renderSelect({
    blocked: reassignmentBlocker({ activeJob: { status: 'running' }, pendingExperimentId: '' }),
  })

  expect(await picker()).toBeDisabled()
  expect(screen.getByText('AI 작업이 끝나면 말투를 바꿀 수 있어요.')).toBeInTheDocument()
})

// The directory is stale: the post service does not know the voice, and answers NotFound rather
// than guessing.
it('reports a refused reassignment under the field and keeps the old voice', async () => {
  const user = userEvent.setup()
  const { onSelect } = renderSelect()
  onSelect.mockRejectedValueOnce(connectAppError('VOICE_NOT_FOUND', Code.NotFound))

  await user.click(within(await pickReview(user)).getByRole('button', { name: '말투 변경' }))

  expect(onSelect).toHaveBeenCalledWith('voice-review')
  expect(await screen.findByRole('alert')).toHaveTextContent('고른 말투를 찾을 수 없어요')
  const field = await picker()
  expect(field).toHaveTextContent('기본 말투')
  await waitFor(() => expect(field).toBeEnabled())
})
