import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { FakeTemplateRow, FakeTemplatesOptions } from '@/test/templates'

const USER = { id: 'alice' }

const QUIET: FakeTemplateRow = {
  id: 'template-quiet',
  name: '짧은 소식',
  body: '<write>인트로를 씁니다</write>',
}

const SHAPED: FakeTemplateRow = {
  id: 'template-review',
  name: '정보성 식당 리뷰',
  body: '<write>인트로를 씁니다</write>',
  targetLength: 1800,
  tagCount: 7,
}

function renderTemplate(path: string, options: FakeTemplatesOptions = {}) {
  return renderAppAt(path, {
    user: USER,
    templates: { templates: [QUIET, SHAPED], ...options },
  })
}

/** TEMPLATE-49: the two numbers are authored on the template screen, behind their own 사용 tick,
 *  as part of the one draft behind the one 저장. */
describe("a template's generation numbers", () => {
  it('opens a stored template with both numbers ticked and shown', async () => {
    renderTemplate('/templates/template-review')

    expect(await screen.findByLabelText('목표 글자 수 사용')).toBeChecked()
    expect(screen.getByLabelText('태그 수 사용')).toBeChecked()
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue(1800)
    expect(screen.getByLabelText('태그 수')).toHaveValue(7)
  })

  it('shows a template with no opinion as unticked, with no field at all', async () => {
    renderTemplate('/templates/template-quiet')

    expect(await screen.findByLabelText('목표 글자 수 사용')).not.toBeChecked()
    expect(screen.getByLabelText('태그 수 사용')).not.toBeChecked()
    expect(screen.queryByLabelText('목표 글자 수')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('태그 수')).not.toBeInTheDocument()
  })

  // POST-20's rule, applied to both: ticking reveals a USABLE number, and a value typed in this
  // session outranks the default when the tick returns.
  it('fills the default on ticking and keeps what was typed across an untick', async () => {
    const user = userEvent.setup()
    renderTemplate('/templates/template-quiet')

    await user.click(await screen.findByLabelText('목표 글자 수 사용'))
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue(1000)

    await user.clear(screen.getByLabelText('목표 글자 수'))
    await user.type(screen.getByLabelText('목표 글자 수'), '2500')
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    expect(screen.queryByLabelText('목표 글자 수')).not.toBeInTheDocument()

    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue(2500)

    await user.click(screen.getByLabelText('태그 수 사용'))
    expect(screen.getByLabelText('태그 수')).toHaveValue(4)
  })

  it('makes the draft dirty and sends both numbers in the one save', async () => {
    const user = userEvent.setup()
    const updates: FakeTemplatesOptions['updates'] = []
    renderTemplate('/templates/template-quiet', { updates })

    const save = await screen.findByRole('button', { name: '저장' })
    expect(save).toBeDisabled()

    await user.click(screen.getByLabelText('태그 수 사용'))
    await waitFor(() => expect(save).toBeEnabled())
    await user.clear(screen.getByLabelText('태그 수'))
    await user.type(screen.getByLabelText('태그 수'), '6')
    await user.click(save)

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0].tagCount).toBe(6)
    // The length was never ticked, so it goes out absent — which is what clears it server-side.
    expect(updates[0].targetLength).toBeUndefined()
  })

  it('clears a stored number by unticking it', async () => {
    const user = userEvent.setup()
    const updates: FakeTemplatesOptions['updates'] = []
    renderTemplate('/templates/template-review', { updates })

    await user.click(await screen.findByLabelText('태그 수 사용'))
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0].tagCount).toBeUndefined()
    expect(updates[0].targetLength).toBe(1800)
  })

  it('blocks the save on a number outside the range and says so under the field', async () => {
    const user = userEvent.setup()
    const updates: FakeTemplatesOptions['updates'] = []
    renderTemplate('/templates/template-review', { updates })

    const tags = await screen.findByLabelText('태그 수')
    await user.clear(tags)
    await user.type(tags, '99')

    expect(screen.getByText('1에서 10 사이로 적어 주세요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    expect(updates).toHaveLength(0)

    // An emptied field is not 의견 없음 either: the tick is how a template says that.
    await user.clear(tags)
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  })

  // TEMPLATE-49: the numbers are not part of the body, so the composition never shows them.
  it('keeps the numbers out of the composition and the source view', async () => {
    const user = userEvent.setup()
    renderTemplate('/templates/template-review')

    await user.click(await screen.findByRole('tab', { name: '원문' }))
    expect(screen.getByLabelText('원문')).toHaveValue(SHAPED.body)
  })
})
