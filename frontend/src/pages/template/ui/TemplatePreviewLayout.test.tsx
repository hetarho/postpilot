import { describe, expect, it } from 'vitest'
import { fireEvent, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { openTemplateDirect } from '@/test/authoring-ui'
const preview = () => within(screen.getByRole('article', { name: '미리보기' }))
// A template is made by talking to the AI; editing its 원문 by hand is the secondary path, and
// its preview is the same live draft the conversation shows.
const PLAIN = {
  id: 'plain',
  name: '빈 구성',
  description: '',
  titleArea: '',
  body: '<write>인사</write>',
}
async function mount(user: ReturnType<typeof userEvent.setup>) {
  renderAppAt('/templates/plain', { user: { id: 'alice' }, templates: { templates: [PLAIN] } })
  await screen.findByRole('log', { name: '템플릿 대화' })
  await openTemplateDirect(user)
}
describe('live shared template preview', () => {
  it('redraws 원문 edits and retains its last valid parse while the text is incomplete', async () => {
    const user = userEvent.setup()
    await mount(user)
    fireEvent.change(screen.getByLabelText('원문'), {
      target: { value: '<write>메뉴 소개</write>' },
    })
    expect(preview().getByText('메뉴 소개')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('원문'), {
      target: { value: '<write>메뉴 소개</write>\n안녕하세요' },
    })
    expect(preview().getByText('안녕하세요')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('원문'), { target: { value: '<write>unfinished' } })
    expect(preview().getByText('안녕하세요')).toBeInTheDocument()
  })
  it('has phone input/preview tabs and keeps the same controlled editor mounted across panes', async () => {
    const user = userEvent.setup()
    await mount(user)
    const name = screen.getByLabelText('이름')
    await user.clear(name)
    await user.type(name, '보관할 이름')
    const tabs = screen.getByRole('tablist', { name: '편집과 미리 보기' })
    expect(tabs).toHaveClass('lg:hidden')
    await user.click(within(tabs).getByRole('tab', { name: '미리 보기' }))
    expect(name).toHaveValue('보관할 이름')
    expect(name.closest('form')).toHaveClass('hidden', 'lg:block')
    await user.click(within(tabs).getByRole('tab', { name: '편집' }))
    expect(screen.getByLabelText('이름')).toBe(name)
    expect(name.closest('form')).not.toHaveClass('hidden')
  })
})
