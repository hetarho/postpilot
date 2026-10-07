import { describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { publishAuthoringDraft } from '@/test/authoring-ui'
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
  await user.click(await screen.findByRole('button', { name: /리뷰.*직접 편집하기/ }))
  await screen.findByLabelText('이름')
}
describe('owner template generation numbers in shared drafts', () => {
  it('captures both stored numbers, outside the source composition', async () => {
    const user = userEvent.setup()
    await mount(user)
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeChecked()
    expect(screen.getByLabelText('최대 태그 수 사용')).toBeChecked()
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue('1800')
    expect(screen.getByLabelText('최대 태그 수')).toHaveValue('7')
    await user.click(screen.getByRole('tab', { name: '원문' }))
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
    await user.click(screen.getByLabelText('최대 태그 수 사용'))
    expect(screen.getByLabelText('최대 태그 수')).toHaveValue('4')
  })
  it('clears only the explicit unticked number in publication and preserves its peer', async () => {
    const user = userEvent.setup()
    const updates: NonNullable<FakeTemplatesOptions['updates']> = []
    await mount(user, true, { updates })
    await user.click(screen.getByLabelText('최대 태그 수 사용'))
    await publishAuthoringDraft(user)
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
      fireEvent.change(screen.getByLabelText('최대 태그 수'), { target: { value } })
      expect(screen.getByText('1에서 10 사이로 적어 주세요.')).toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: '편집 내용 보관하고 확인하기' }))
      await waitFor(() => expect(patches).toHaveLength(1))
      expect(patches[0].tagCount).toBe(value || ' ')
      expect(updates).toHaveLength(0)
      expect(screen.queryByRole('button', { name: '저장할 내용 확인하기' })).not.toBeInTheDocument()
    },
  )
  it('keeps the owner’s earlier numeric input across an untick and AI/direct method switch', async () => {
    const user = userEvent.setup()
    await mount(user, false)
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    fireEvent.change(screen.getByLabelText('목표 글자 수'), { target: { value: '2500' } })
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    await user.click(screen.getByRole('button', { name: /AI로 편집하기$/ }))
    await screen.findByRole('heading', { name: '어떤 점을 바꿔 볼까요?' })
    await user.click(screen.getByRole('button', { name: /직접 편집하기$/ }))
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    expect(screen.getByLabelText('목표 글자 수')).toHaveValue('2500')
  })
})
