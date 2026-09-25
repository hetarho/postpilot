import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code } from '@connectrpc/connect'
import { afterEach, expect, it, vi } from 'vitest'
import { connectAppError } from '@/test/app-error'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { reassignmentBlocker } from '../model/reassignment'
import { PostVoiceSelect } from './PostVoiceSelect'

afterEach(cleanup)

const TWO_VOICES = [
  { id: 'voice-default', name: '기본 말투', isDefault: true },
  { id: 'voice-review', name: '리뷰' },
]
const CURRENT = {
  id: 'voice-default',
  name: '기본 말투',
  deleted: false,
  sourceLanguage: 'ko' as const,
}

function renderSelect(props: Partial<Parameters<typeof PostVoiceSelect>[0]> = {}) {
  const onSelect = vi.fn<(voiceId: string) => Promise<void>>(async () => {})
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    voice: { voices: TWO_VOICES },
  })
  render(
    <PostVoiceSelect
      ownerId="alice"
      value="voice-default"
      current={CURRENT}
      confirm
      onSelect={onSelect}
      {...props}
    />,
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  return { onSelect }
}

const picker = () => screen.findByRole('combobox', { name: /말투/ })

/** The directory answers after the first paint: choose only once the field is live. */
async function pickReview(user: ReturnType<typeof userEvent.setup>) {
  const field = await picker()
  await waitFor(() => expect(field).toBeEnabled())
  await user.click(field)
  await user.click(await screen.findByRole('option', { name: '리뷰' }))
  return screen.findByRole('dialog', { name: '말투를 바꿀까요?' })
}

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
