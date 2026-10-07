import { describe, expect, it } from 'vitest'
import { fireEvent, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
const preview = () => within(screen.getByRole('article', { name: '미리보기' }))
async function mount(user: ReturnType<typeof userEvent.setup>) {
  renderAppAt('/templates/new', { user: { id: 'alice' }, templates: { templates: [] } })
  await user.click(await screen.findByRole('button', { name: '직접 편집' }))
  await screen.findByLabelText('이름')
}
describe('live shared template preview', () => {
  it('redraws builder/source edits and retains its last valid parse while source is incomplete', async () => {
    const user = userEvent.setup()
    await mount(user)
    await user.click(
      within(screen.getByRole('group', { name: '블록 추가' })).getByRole('button', {
        name: /^AI가 쓰는 글/,
      }),
    )
    await user.type(screen.getByLabelText('이 자리에 오는 것'), '메뉴 소개')
    expect(preview().getByText('메뉴 소개')).toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: '원문' }))
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
