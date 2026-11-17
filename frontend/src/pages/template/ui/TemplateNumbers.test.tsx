import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { openTemplateDirect, publishTemplateConversation } from '@/test/authoring-ui'
import type { FakeTemplatesOptions } from '@/test/templates'
import type { FakeAuthoringOptions } from '@/test/authoring'
const template = {
  id: 'review',
  name: '리뷰',
  body: '<write>메뉴 소개</write>',
  targetLength: 1800,
  tagCount: 7,
}
async function mount(
  user: ReturnType<typeof userEvent.setup>,
  numbers = true,
  templates: FakeTemplatesOptions = {},
  authoring: FakeAuthoringOptions = {},
) {
  renderAppAt('/templates/review', {
    user: { id: 'alice' },
    templates: {
      templates: [
        numbers ? template : { ...template, targetLength: undefined, tagCount: undefined },
      ],
      ...templates,
    },
    authoring,
  })
  await openTemplateDirect(user)
}
describe('owner template generation numbers in shared drafts', () => {
  it('captures both stored numbers, outside the source composition', async () => {
    const user = userEvent.setup()
    await mount(user)
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeChecked()
    expect(screen.getByLabelText('최소 태그 수 사용')).toBeChecked()
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue('1800')
    expect(screen.getByLabelText('최소 태그 수')).toHaveValue('7')
    // The text is edited as 원문 alone: there is no builder to drag rows in.
    expect(screen.queryByRole('button', { name: '고급' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('원문')).toHaveValue(template.body)
  })
  it('uses owner opt-in defaults and remembers typed numbers when toggled off and on', async () => {
    const user = userEvent.setup()
    await mount(user, false)
    expect(screen.getByLabelText('목표 글자 수 사용')).not.toBeChecked()
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue('1000')
    fireEvent.change(screen.getByLabelText('목표 글자 수'), { target: { value: '2500' } })
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    expect(screen.queryByLabelText('목표 글자 수')).not.toBeInTheDocument()
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue('2500')
    await user.click(screen.getByLabelText('최소 태그 수 사용'))
    expect(screen.getByLabelText('최소 태그 수')).toHaveValue('4')
  })
  it('clears only the explicit unticked number in publication and preserves its peer', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeTemplatesOptions['updates']> = []
    await mount(user, true, { updates })
    await user.click(screen.getByLabelText('최소 태그 수 사용'))
    await publishTemplateConversation(user)
    await waitFor(() => expect(updates).toHaveLength(1))
    expect(updates[0]).toMatchObject({ targetLength: 1800, tagCount: undefined })
  })
  it.each(['99', '', '-2'])(
    'retains invalid enabled numeric input %j and blocks canonical publication',
    async (value) => {
      const user = userEvent.setup()
      const patches: NonNullable<FakeAuthoringOptions['patches']> = []
      const updates: NonNullable<FakeTemplatesOptions['updates']> = []
      await mount(user, true, { updates }, { patches })
      fireEvent.change(screen.getByLabelText('최소 태그 수'), { target: { value } })
      expect(screen.getByText('1에서 10 사이로 적어 주세요.')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
      await waitFor(() => expect(patches).toHaveLength(1))
      expect(patches[0].tagCount).toBe(value || ' ')
      expect(updates).toHaveLength(0)
      expect(
        screen.queryByRole('button', { name: /^(변경사항 저장|새 템플릿으로 저장)$/ }),
      ).toBeNull()
    },
  )
  it('keeps the owner’s earlier numeric input across an untick and conversation/direct switch', async () => {
    const user = userEvent.setup()
    await mount(user, false)
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    fireEvent.change(screen.getByLabelText('목표 글자 수'), { target: { value: '2500' } })
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    // Back to the conversation and into the text again: the same working draft.
    await user.click(screen.getByRole('button', { name: /AI로 편집하기$/ }))
    await screen.findByRole('log', { name: '템플릿 대화' })
    await openTemplateDirect(user)
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue('2500')
  })
})
