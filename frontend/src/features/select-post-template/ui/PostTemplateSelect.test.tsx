import { useState } from 'react'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import type { FakeTemplatesOptions } from '@/test/templates'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { PostTemplateSelect } from './PostTemplateSelect'

afterEach(cleanup)

const PURPOSES = [
  { id: 'template-review', name: '정보성 식당 리뷰', description: '협찬 방문 리뷰' },
  { id: 'template-diary', name: '일기' },
]

/** The select as the dock holds it: the value follows the choice the caller accepted. */
function renderSelect(
  props: Partial<Parameters<typeof PostTemplateSelect>[0]> = {},
  templates: FakeTemplatesOptions = { templates: PURPOSES },
) {
  const onSelect = vi.fn()
  function Harness() {
    const [value, setValue] = useState(props.value ?? '')
    return (
      <PostTemplateSelect
        ownerId="alice"
        {...props}
        value={value}
        onSelect={(id) => {
          onSelect(id)
          setValue(id)
        }}
      />
    )
  }
  const transport = createFakeAuthTransport({ user: { id: 'alice' }, templates })
  render(<Harness />, { wrapper: withProviders(transport, createTestQueryClient()) })
  return { onSelect }
}

const picker = () => screen.findByRole('combobox', { name: /템플릿/ })

// A failed directory read must not be indistinguishable from "you have no 템플릿" — the select
// would be enabled with 없음 alone, and clearing would be the only thing left to do.
it('says so and offers a retry when the directory cannot be read', async () => {
  renderSelect({}, { listFails: true })

  const field = await picker()
  expect(await screen.findByText(/템플릿 목록을 불러오지 못했어요/)).toBeInTheDocument()
  expect(field).toBeDisabled()
  expect(screen.getByRole('button', { name: '다시 시도' })).toBeInTheDocument()
})

it('stays usable during a job and says the running one keeps its own brief', async () => {
  renderSelect({
    value: 'template-review',
    current: { id: 'template-review', name: '정보성 식당 리뷰' },
    jobRunning: true,
  })

  const field = await picker()
  await waitFor(() => expect(field).toHaveTextContent('정보성 식당 리뷰'))
  expect(field).toBeEnabled()
  expect(
    await screen.findByText(/진행 중인 AI 작업은 시작할 때의 템플릿으로 끝나요/),
  ).toBeInTheDocument()
})

it('reads 없음 with 없음 selected when nothing is assigned', async () => {
  const user = userEvent.setup()
  renderSelect()

  const field = await picker()
  expect(field).toHaveTextContent('없음')
  await user.click(field)
  expect(screen.getByRole('option', { name: '없음', selected: true })).toBeInTheDocument()
})

// The dock row is three controls wide on a 360px screen, so the chosen 템플릿's own brief and the
// way to the 템플릿 page are not on it: both belong to the directory that owns them.
it('shows no description and no 템플릿 관리 link', async () => {
  const user = userEvent.setup()
  const { onSelect } = renderSelect()

  const field = await picker()
  await waitFor(() => expect(field).toBeEnabled())
  await user.click(field)
  await user.click(await screen.findByRole('option', { name: '정보성 식당 리뷰' }))

  expect(onSelect).toHaveBeenCalledWith('template-review')
  await waitFor(() => expect(field).toHaveTextContent('정보성 식당 리뷰'))
  expect(screen.queryByText('협찬 방문 리뷰')).not.toBeInTheDocument()
  expect(screen.queryByRole('link', { name: '템플릿 관리' })).not.toBeInTheDocument()
})
