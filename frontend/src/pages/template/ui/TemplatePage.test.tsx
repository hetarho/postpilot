import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeTemplateRow, FakeTemplatesOptions } from '@/test/templates'

const USER = { id: 'alice' }

const REVIEW: FakeTemplateRow = {
  id: 'template-review',
  name: '정보성 식당 리뷰',
  description: '협찬 방문 리뷰',
  body: '<write>인트로를 씁니다</write>\n<slot kind="place" label="네이버 지도"/>',
  postCount: 2,
}

function renderTemplate(path: string, templates: FakeTemplatesOptions = {}, calls: string[] = []) {
  return renderAppAt(path, {
    user: USER,
    calls,
    templates: { templates: [REVIEW], ...templates },
  })
}

describe('the template screen', () => {
  // A3: a row's target loads the stored template into one draft.
  it('opens a stored template with its name, description and composition', async () => {
    renderTemplate('/templates/template-review')

    expect(await screen.findByLabelText('이름')).toHaveValue('정보성 식당 리뷰')
    expect(screen.getByLabelText(/어떤 글인가요/)).toHaveValue('협찬 방문 리뷰')
    // The composition reads as the outline: one row per block, by its own text.
    expect(screen.getByText('인트로를 씁니다')).toBeInTheDocument()
    expect(screen.getByText('네이버 지도')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '← 템플릿 목록' })).toHaveAttribute(
      'href',
      '/templates',
    )
  })

  // A4: one save for the whole screen, disabled until something actually differs.
  it('disables the save until the draft differs, then writes all three fields in one call', async () => {
    const user = userEvent.setup()
    const updates: FakeTemplatesOptions['updates'] = []
    renderTemplate('/templates/template-review', { updates })

    const save = await screen.findByRole('button', { name: '저장' })
    expect(save).toBeDisabled()

    await user.type(screen.getByLabelText('이름'), ' 2편')
    await waitFor(() => expect(save).toBeEnabled())
    await user.click(save)

    await waitFor(() => expect(updates).toHaveLength(1))
    // All three present in ONE call: they are one decision now, not three saves.
    expect(updates[0]).toEqual({
      id: 'template-review',
      name: '정보성 식당 리뷰 2편',
      description: '협찬 방문 리뷰',
      body: REVIEW.body,
    })
    expect(await screen.findByText('저장했어요.')).toBeInTheDocument()
  })

  // A11: an untouched composition round-trips byte for byte, so a template saved before this
  // change and one saved after carry the identical body.
  it('saves the stored body unchanged when only the name was edited', async () => {
    const user = userEvent.setup()
    const updates: FakeTemplatesOptions['updates'] = []
    renderTemplate('/templates/template-review', { updates })

    await user.type(await screen.findByLabelText('이름'), '!')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0].body).toBe(REVIEW.body)
  })

  // A2: /templates/new is the same screen with nothing in it, and abandoning it writes nothing.
  it('opens an empty draft at /templates/new and writes nothing until saved', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const creates: FakeTemplatesOptions['creates'] = []
    renderTemplate('/templates/new', { creates }, calls)

    expect(await screen.findByRole('heading', { name: '새 템플릿' })).toBeInTheDocument()
    expect(screen.getByLabelText('이름')).toHaveValue('')
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()

    // Leaving without saving creates nothing — the guard does not even fire on a clean draft.
    await user.click(screen.getByRole('link', { name: '← 템플릿 목록' }))
    await screen.findByRole('heading', { level: 1, name: '템플릿' })
    expect(creates).toHaveLength(0)
    expect(calls).not.toContain('CreateTemplate')
  })

  // A2 + A4: the same screen creates, and the save carries all three fields.
  it('creates from the empty screen and lands on the saved template', async () => {
    const user = userEvent.setup()
    const creates: FakeTemplatesOptions['creates'] = []
    renderTemplate('/templates/new', { creates })

    await user.type(await screen.findByLabelText('이름'), '카페 방문기')
    await user.type(screen.getByLabelText(/어떤 글인가요/), '동네 카페')
    await user.click(screen.getByRole('button', { name: '작성' }))
    await user.type(screen.getByLabelText('무엇을 쓸지'), '첫인상을 씁니다')

    await user.click(screen.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(creates).toHaveLength(1))
    expect(creates[0]).toEqual({
      name: '카페 방문기',
      description: '동네 카페',
      body: '<write>첫인상을 씁니다</write>',
    })
  })

  // The save's baseline comes from the mutation's OWN response, not from the directory query
  // that lags it by a refetch — otherwise the screen stays dirty after a save, which re-enables
  // 저장 and makes the guard warn about a template that was just written.
  it('goes clean the moment a save lands, without waiting for the directory to catch up', async () => {
    const user = userEvent.setup()
    renderTemplate('/templates/template-review')

    await user.type(await screen.findByLabelText('이름'), '!')
    const save = screen.getByRole('button', { name: '저장' })
    await user.click(save)

    await screen.findByText('저장했어요.')
    expect(save).toBeDisabled()
    // And leaving asks nothing, because there is nothing unsaved to lose.
    await user.click(screen.getByRole('link', { name: '← 템플릿 목록' }))
    expect(await screen.findByRole('heading', { level: 1, name: '템플릿' })).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  // The same window, on the create path: the screen's own redirect must not be intercepted by
  // its own leave guard, and the template it lands on must not flash "not found" while the
  // directory refetches.
  it('lands on the created template without a guard or a not-found flash', async () => {
    const user = userEvent.setup()
    const creates: FakeTemplatesOptions['creates'] = []
    renderTemplate('/templates/new', { creates })

    await user.type(await screen.findByLabelText('이름'), '카페 방문기')
    await user.click(screen.getByRole('button', { name: '작성' }))
    await user.type(screen.getByLabelText('무엇을 쓸지'), '첫인상을 씁니다')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(creates).toHaveLength(1))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.queryByText(/이 템플릿을 찾을 수 없어요/)).not.toBeInTheDocument()
    // The heading is the saved template's name, so the screen really did land on it.
    expect(await screen.findByRole('heading', { name: '카페 방문기' })).toBeInTheDocument()
  })

  // A11 at the wire: a stored body carrying significant outer bytes must not read as dirty on
  // open, and must not be silently rewritten by a save of some other field.
  it('never trims the stored body', async () => {
    const user = userEvent.setup()
    const updates: FakeTemplatesOptions['updates'] = []
    renderTemplate('/templates/template-padded', {
      templates: [{ id: 'template-padded', name: '여백', body: '\n인트로\n' }],
      updates,
    })

    // Untouched, it is not dirty — the body is what was stored, byte for byte.
    expect(await screen.findByRole('button', { name: '저장' })).toBeDisabled()

    await user.type(screen.getByLabelText('이름'), '!')
    await user.click(screen.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0].body).toBe('\n인트로\n')
  })

  // A4: leaving with unsaved changes warns first.
  it('warns before leaving with unsaved changes', async () => {
    const user = userEvent.setup()
    renderTemplate('/templates/template-review')

    await user.type(await screen.findByLabelText('이름'), '!')
    await user.click(screen.getByRole('link', { name: '← 템플릿 목록' }))

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/지금 나가면 사라집니다/)).toBeInTheDocument()
    await user.click(dialog.getByRole('button', { name: '저장하지 않고 나가기' }))
    expect(await screen.findByRole('heading', { level: 1, name: '템플릿' })).toBeInTheDocument()
  })

  // A10: a body the parser cannot read is neither guessed at nor silently dropped.
  it('says an unreadable composition cannot be read and offers only to start it over', async () => {
    const user = userEvent.setup()
    const updates: FakeTemplatesOptions['updates'] = []
    renderTemplate('/templates/template-broken', {
      templates: [{ id: 'template-broken', name: '옛 템플릿', body: '<write>닫히지 않음' }],
      updates,
    })

    expect(await screen.findByText(/구성을 읽을 수 없어요/)).toBeInTheDocument()
    // The name and the description are still editable — only the composition is unreadable.
    expect(screen.getByLabelText('이름')).toHaveValue('옛 템플릿')
    // No grammar is shown even here: there is no source view to fall back to (A9).
    expect(document.body.textContent ?? '').not.toContain('<write>')

    await user.click(screen.getByRole('button', { name: '구성 비우고 다시 만들기' }))
    // Clearing writes nothing: the screen's save is still the only write.
    expect(updates).toHaveLength(0)
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  })

  // A12: nothing on this screen calls a provider or enqueues a job.
  it('calls no provider and enqueues nothing', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderTemplate('/templates/template-review', {}, calls)

    await user.type(await screen.findByLabelText('이름'), '!')
    await user.click(screen.getByRole('button', { name: '저장' }))
    await screen.findByText('저장했어요.')

    const allowed = ['GetMe', 'ListTemplates', 'UpdateTemplate']
    expect(calls.filter((call) => !allowed.includes(call))).toEqual([])
  })
})
