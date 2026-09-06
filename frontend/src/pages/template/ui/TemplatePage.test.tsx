import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeTemplateRow, FakeTemplatesOptions } from '@/test/templates'

const USER = { id: 'alice' }

/** A palette button by its name, scoped to the toolbar: a row's badge carries the same name as
 *  the button that creates it, and the accessible name now includes the button's visible help. */
const paletteButton = (name: string) =>
  within(screen.getByRole('group', { name: '블록 추가' })).getByRole('button', {
    name: new RegExp(`^${name}`),
  })

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
  // TEMPLATE-37: a stored place or link position opens as 고정 문구 rather than making the
  // composition unreadable — and reading it is NOT an edit. The editor emits nothing until the
  // user changes something, so the body stays byte-identical and 저장 stays disabled; the
  // migration to literal text rides the next real save.
  it('opens a legacy position as fixed text without making the draft dirty', async () => {
    const user = userEvent.setup()
    renderTemplate('/templates/template-review')

    expect(await screen.findByLabelText('이름')).toHaveValue('정보성 식당 리뷰')
    // The retired position reads as fixed text carrying its label.
    expect(screen.getByText('네이버 지도')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()

    // One real edit, and only then does the save open — writing the row back as literal text.
    await user.click(paletteButton('고정 문구'))
    await user.type(screen.getByLabelText('들어갈 문구'), '지도는 아래에')
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
  })

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
    await user.click(paletteButton('AI가 쓰는 글'))
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
    await user.click(paletteButton('AI가 쓰는 글'))
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

  // TEMPLATE-42: import IS pasting. What the AI wrote is what gets stored — byte for byte, outer
  // whitespace and all — and the save carries the identical string.
  it('saves a pasted body byte for byte', async () => {
    const user = userEvent.setup()
    const updates: FakeTemplatesOptions['updates'] = []
    renderTemplate('/templates/template-review', { updates })

    expect(await screen.findByLabelText('이름')).toHaveValue('정보성 식당 리뷰')
    await user.click(screen.getByRole('tab', { name: '원문' }))

    const source = screen.getByLabelText('원문')
    await user.clear(source)
    await user.click(source)
    await user.paste('  <write>붙여넣은 본문</write>\n')

    await user.click(screen.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0].body).toBe('  <write>붙여넣은 본문</write>\n')
  })

  // TEMPLATE-30 / TEMPLATE-7: a body that does not parse cannot be saved from EITHER mode, and
  // editing the name does not buy a way past it.
  it('refuses to save an unparsable body even after the name is edited', async () => {
    const user = userEvent.setup()
    renderTemplate('/templates/template-broken', {
      templates: [{ id: 'template-broken', name: '옛 템플릿', body: '<write>닫히지 않음' }],
    })

    const name = await screen.findByLabelText('이름')
    await user.type(name, ' 고침')
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()

    // And from the source mode, where the reason is finally visible.
    await user.click(screen.getByRole('tab', { name: '원문' }))
    expect(screen.getByRole('alert')).toHaveTextContent('1번째 줄: 닫히지 않았어요')
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()

    // Closing the tag is what opens the save.
    await user.type(screen.getByLabelText('원문'), '</write>')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
  })

  // TEMPLATE-30: the unreadable state now has a way to FIX rather than only a way to discard.
  it('sends 원문에서 고치기 to the source with the caret in the text and the error shown', async () => {
    const user = userEvent.setup()
    renderTemplate('/templates/template-broken', {
      templates: [{ id: 'template-broken', name: '옛 템플릿', body: '<write>닫히지 않음' }],
    })

    await screen.findByText(/구성을 읽을 수 없어요/)
    await user.click(screen.getByRole('button', { name: '원문에서 고치기' }))

    const source = screen.getByLabelText('원문')
    expect(source).toHaveValue('<write>닫히지 않음')
    expect(source).toHaveFocus()
    expect(screen.getByRole('alert')).toHaveTextContent('1번째 줄')
    // Nothing was written: the screen's 저장 is still the only write.
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  })

  // The two modes are two renderings of ONE field: what one writes is what the other shows.
  it('round-trips builder → source → builder without changing the composition', async () => {
    const user = userEvent.setup()
    renderTemplate('/templates/template-review')

    await screen.findByLabelText('이름')
    await user.click(screen.getByRole('tab', { name: '원문' }))
    expect(screen.getByLabelText('원문')).toHaveValue(REVIEW.body)

    await user.click(screen.getByRole('tab', { name: '블록' }))
    // The outline is seeded from the same body, retired position included (TEMPLATE-37).
    expect(screen.getByText('네이버 지도')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  })
})
