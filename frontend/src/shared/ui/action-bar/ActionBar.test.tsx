import { render, screen } from '@testing-library/react'
import { ActionBar } from './ActionBar'

describe('ActionBar', () => {
  // THEME-24: a list's add action is docked at every width. The regression this pins is the
  // retired phone-only dock, which undid the position, plane, corner and shadow from `sm:` up
  // and so let a long list carry 새 글 below the fold on a desk.
  it('keeps the list dock sticky and raised at every width', () => {
    render(
      <ActionBar dock="list" ariaLabel="글 작성">
        <button>새 글</button>
      </ActionBar>,
    )

    const bar = screen.getByLabelText('글 작성')
    expect(bar).toHaveClass('sticky', 'rounded-xl', 'shadow-md', 'bg-surface-highest')
    for (const reset of [
      'sm:static',
      'sm:rounded-none',
      'sm:shadow-none',
      'sm:bg-transparent',
      'sm:p-0',
    ]) {
      expect(bar.className).not.toContain(reset)
    }
  })

  // The other half of the same decision: above the phone the bar is only as wide as what it
  // holds, so it is not the full-width card with one left-aligned button.
  it('shrinks the list dock to its contents above the phone, and never the column-spanning one', () => {
    const { rerender } = render(
      <ActionBar dock="list" ariaLabel="글 작성">
        <button>새 글</button>
      </ActionBar>,
    )
    expect(screen.getByLabelText('글 작성')).toHaveClass('sm:w-fit', 'sm:ml-auto')

    rerender(
      <ActionBar dock="always" ariaLabel="초안">
        <button>생성</button>
      </ActionBar>,
    )
    const spanning = screen.getByLabelText('초안')
    expect(spanning).toHaveClass('sticky')
    expect(spanning.className).not.toContain('sm:w-fit')
  })
})
