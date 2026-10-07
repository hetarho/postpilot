import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createPortal } from 'react-dom'
import { Button } from '../button/Button'
import { buttonStyles } from '../button/buttonStyles'
import { ActionBar } from './ActionBar'

describe('ActionBar', () => {
  // THEME-24: a list's add action is docked at every width — the regression this pins is the
  // retired phone-only dock, which undid the position from `sm:` up and so let a long list carry
  // 새 글 below the fold on a desk — and it docks with NO plane of its own, the control floating
  // over the rows on its own shadow rather than sitting on a card holding one button.
  it('floats the list dock at every width with no plane of its own', () => {
    render(
      <ActionBar dock="list" ariaLabel="글 작성">
        <button>새 글</button>
      </ActionBar>,
    )

    const bar = screen.getByLabelText('글 작성')
    expect(bar).toHaveClass('sticky', 'ml-auto', 'w-fit', '*:shadow-lg')
    expect(bar.className).not.toMatch(/bg-surface-highest|rounded|shadow-md|(?:^|[\s:])p-\d/)
    for (const reset of ['sm:static', 'sm:ml-0', 'sm:w-full']) {
      expect(bar.className).not.toContain(reset)
    }
  })

  it('fits and centers an action group without stretching across its column', () => {
    render(
      <ActionBar dock="always" ariaLabel="초안">
        <button>생성</button>
      </ActionBar>,
    )

    const bar = screen.getByLabelText('초안')
    expect(bar).toHaveClass('sticky', 'rounded-xl', 'shadow-md', 'w-fit', 'max-w-full', 'mx-auto')
    expect(bar).toHaveClass('self-center', 'p-2')
    expect(bar).toHaveAttribute('data-action-bar', 'always')
    expect(bar).not.toHaveClass('bg-surface-highest', 'w-full', 'sm:p-4')
  })

  it('provides bounded readable space when the dock contains a composer', () => {
    render(
      <ActionBar width="content" ariaLabel="수정 작업">
        <textarea aria-label="수정 요청" />
        <Button>수정</Button>
      </ActionBar>,
    )

    const bar = screen.getByLabelText('수정 작업')
    expect(bar).toHaveClass('w-full', 'max-w-measure', 'mx-auto', 'self-center')
    expect(bar).not.toHaveClass('w-fit')
    expect(bar).toContainElement(screen.getByRole('textbox', { name: '수정 요청' }))
  })

  it('preserves the mounted action and its accessible label while work is pending', async () => {
    const user = userEvent.setup()
    const started = vi.fn()
    const action = (pending: boolean) => (
      <ActionBar ariaLabel="저장 작업">
        <Button pending={pending} onClick={started}>
          저장하기
        </Button>
      </ActionBar>
    )
    const { rerender } = render(action(false))
    const button = screen.getByRole('button', { name: '저장하기' })
    await user.tab()
    expect(button).toHaveFocus()
    await user.click(button)
    expect(started).toHaveBeenCalledOnce()

    rerender(action(true))
    expect(screen.getByRole('button', { name: '저장하기' })).toBe(button)
    expect(button).toHaveAttribute('aria-busy', 'true')
    expect(button).toBeDisabled()
    expect(button.querySelector('.opacity-0')).toHaveTextContent('저장하기')
    await user.click(button)
    expect(started).toHaveBeenCalledOnce()

    rerender(action(false))
    expect(screen.getByRole('button', { name: '저장하기' })).toBe(button)
    expect(button).not.toHaveAttribute('aria-busy')
    expect(button).toBeEnabled()
  })

  it('scopes dock targets to its DOM while linked actions and portalled controls keep semantics', () => {
    render(
      <>
        <Button>일반 동작</Button>
        <ActionBar ariaLabel="후보 작업">
          <Button size="compact">이전 단계</Button>
          <Button size="icon" aria-label="안내">
            ?
          </Button>
          <a href="/tests/history" className={buttonStyles({ variant: 'cta' })}>
            작업 내역
          </a>
          {createPortal(<Button>대화상자 동작</Button>, document.body)}
        </ActionBar>
      </>,
    )

    for (const target of [
      screen.getByRole('button', { name: '이전 단계' }),
      screen.getByRole('button', { name: '안내' }),
      screen.getByRole('link', { name: '작업 내역' }),
    ]) {
      expect(target.closest('[data-action-bar]')).toBe(screen.getByLabelText('후보 작업'))
      expect(target).toHaveClass('ui-button')
    }
    expect(screen.getByRole('button', { name: '안내' })).toHaveClass('ui-button-icon')
    expect(screen.getByRole('link', { name: '작업 내역' })).toHaveAttribute(
      'href',
      '/tests/history',
    )
    for (const name of ['일반 동작', '대화상자 동작']) {
      const button = screen.getByRole('button', { name })
      expect(button.closest('[data-action-bar]')).toBeNull()
      expect(button).toHaveClass('min-h-10', 'pointer-coarse:min-h-11')
    }
  })
})
