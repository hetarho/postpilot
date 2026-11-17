import { describe, expect, it } from 'vitest'
import { fireEvent, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { openConversationDirect } from '@/test/authoring-ui'
const preview = () => within(screen.getByRole('article', { name: '미리보기' }))
// The draft pane beside the conversation is the shared preview; typing the template's text
// directly redraws it.
const PLAIN = {
  id: 'plain',
  name: '빈 구성',
  description: '',
  titleArea: '',
  body: '<write>인사</write>',
}
async function mount(user: ReturnType<typeof userEvent.setup>) {
  renderAppAt('/templates/plain', { user: { id: 'alice' }, templates: { templates: [PLAIN] } })
  await openConversationDirect(user)
  await screen.findByLabelText('이름')
}
describe('live shared template preview', () => {
  it('redraws source edits and retains its last valid parse while source is incomplete', async () => {
    const user = userEvent.setup()
    await mount(user)
    fireEvent.change(screen.getByLabelText('원문'), {
      target: { value: '<write>메뉴 소개</write>\n안녕하세요' },
    })
    expect(preview().getByText('안녕하세요')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('원문'), { target: { value: '<write>unfinished' } })
    expect(preview().getByText('안녕하세요')).toBeInTheDocument()
  })
  it('has phone edit/draft panes and keeps the same controlled editor mounted across them', async () => {
    const user = userEvent.setup()
    await mount(user)
    const name = screen.getByLabelText('이름')
    await user.clear(name)
    await user.type(name, '보관할 이름')
    const panes = screen.getByRole('tablist', { name: '대화와 초안' })
    expect(panes).toHaveClass('@3xl:hidden')
    await user.click(within(panes).getByRole('tab', { name: '초안' }))
    expect(name).toHaveValue('보관할 이름')
    await user.click(within(panes).getByRole('tab', { name: '직접 고치기' }))
    expect(screen.getByLabelText('이름')).toBe(name)
  })
})
