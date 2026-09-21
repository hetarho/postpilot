import { render, screen } from '@testing-library/react'
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

  // The column-spanning dock is the one that keeps a surface: it carries a view's committing
  // controls and their refusals, not one add action.
  it('keeps the column-spanning dock on its own plane', () => {
    render(
      <ActionBar dock="always" ariaLabel="초안">
        <button>생성</button>
      </ActionBar>,
    )

    const spanning = screen.getByLabelText('초안')
    expect(spanning).toHaveClass('sticky', 'rounded-xl', 'shadow-md', 'bg-surface-highest')
    expect(spanning.className).not.toMatch(/w-fit|ml-auto/)
  })
})
