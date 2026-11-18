import { render, screen, within } from '@testing-library/react'
import { WorkRow, WorkRowList } from './WorkRow'

function row(props: Partial<Parameters<typeof WorkRow>[0]> = {}) {
  return (
    <WorkRowList label="rows">
      <WorkRow
        title="A long work title"
        meta={<span>meta line</span>}
        link={({ className, children }) => (
          <a href="/items/1" className={className}>
            {children}
          </a>
        )}
        {...props}
      />
    </WorkRowList>
  )
}

it('is one stretched link holding the title and meta, with a chevron and no tint at rest', () => {
  render(row())
  const item = screen.getByRole('listitem')
  const link = within(item).getByRole('link')
  expect(within(item).getAllByRole('link')).toEqual([link])
  expect(link).toHaveTextContent('A long work titlemeta line')
  expect(link).toHaveClass('after:absolute', 'after:inset-0')
  expect(item).toHaveClass('relative')
  expect(item).not.toHaveClass('bg-badge-accent-bg')
  expect(screen.getByText('A long work title')).toHaveClass('truncate')
})

it('tints the leading row, names its cue before the chevron and draws a leading glyph', () => {
  render(row({ tint: true, cue: 'Continue', leading: <svg data-testid="glyph" /> }))
  const item = screen.getByRole('listitem')
  expect(item).toHaveClass('bg-badge-accent-bg')
  expect(screen.getByRole('link')).toHaveTextContent(/Continue$/)
  expect(screen.getByTestId('glyph').parentElement).toHaveAttribute('aria-hidden', 'true')
})

it('keeps its one trailing action a sibling above the stretched link, never nested', () => {
  render(row({ action: <a href="https://elsewhere.test">Elsewhere</a> }))
  const action = screen.getByRole('link', { name: 'Elsewhere' })
  expect(action.parentElement).toHaveClass('relative', 'z-10')
  expect(action.parentElement?.closest('a')).toBeNull()
  expect(screen.getAllByRole('link')).toHaveLength(2)
})
