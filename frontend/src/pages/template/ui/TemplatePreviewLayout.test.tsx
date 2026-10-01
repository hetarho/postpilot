import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'

const USER = { id: 'alice' }

const preview = () => within(screen.getByRole('article', { name: '미리보기' }))

// TMPL-65, TMPL-67: the preview follows the one draft, beside the composition at lg and one tap
// away below it. jsdom has no media queries, so the breakpoint is read off the classes that
// decide it.
describe('the template preview on the template screen', () => {
  it('redraws on every edit of the draft', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/new', { user: USER, templates: { templates: [] } })
    await screen.findByLabelText('이름')
    expect(
      preview().getByText('구성에 블록을 추가하면 여기에 미리보기가 보여요.'),
    ).toBeInTheDocument()

    await user.click(
      within(screen.getByRole('group', { name: '블록 추가' })).getByRole('button', {
        name: /^AI가 쓰는 글/,
      }),
    )
    await user.type(screen.getByLabelText('이 자리에 오는 것'), '메뉴 소개')
    expect(preview().getByText('메뉴 소개')).toBeInTheDocument()

    // 원문 as typed, too.
    await user.click(screen.getByRole('tab', { name: '원문' }))
    await user.type(screen.getByLabelText('원문'), '\n안녕하세요')
    expect(preview().getByText('안녕하세요')).toBeInTheDocument()
  })

  it('switches between the composition and the preview below lg', async () => {
    const user = userEvent.setup()
    renderAppAt('/templates/new', { user: USER, templates: { templates: [] } })
    await screen.findByLabelText('이름')

    const views = screen.getByRole('tablist', { name: '구성과 미리보기' })
    expect(views).toHaveClass('lg:hidden')
    const editPanel = document.getElementById('template-edit-panel')
    const previewPanel = document.getElementById('template-preview-panel')
    expect(editPanel).not.toHaveClass('hidden')
    expect(previewPanel).toHaveClass('hidden', 'lg:block')
    expect(within(views).getByRole('tab', { name: '구성' })).toHaveAttribute(
      'aria-controls',
      'template-edit-panel',
    )

    await user.click(within(views).getByRole('tab', { name: '미리보기' }))
    expect(editPanel).toHaveClass('hidden', 'lg:block')
    expect(previewPanel).not.toHaveClass('hidden')
    expect(within(views).getByRole('tab', { name: '미리보기' })).toHaveAttribute(
      'aria-controls',
      'template-preview-panel',
    )
    // The fields stay above the switch in either view.
    expect(screen.getByLabelText('이름')).toBeVisible()
    // And it never makes the draft dirty.
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  })
})
