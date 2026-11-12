import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { FieldCount } from './FieldCount'
import { Textarea } from './Textarea'

describe('field count', () => {
  it.each([
    [-3, '3자 초과', true],
    [0, '0자 남음', false],
    [7, '7자 남음', false],
  ] as const)(
    'describes a field at %i remaining and announces only an exceeded bound',
    (left, text, announced) => {
      render(
        <>
          <Textarea aria-label="내용" aria-describedby="count" />
          <FieldCount id="count" left={left} />
        </>,
      )
      expect(screen.getByRole('textbox', { name: '내용' })).toHaveAccessibleDescription(text)
      expect(screen.getByText(text)).toHaveAttribute('id', 'count')
      if (announced) expect(screen.getByRole('status')).toHaveTextContent(text)
      else expect(screen.queryByRole('status')).not.toBeInTheDocument()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    },
  )

  it('keeps the description and typed Unicode draft connected when the counter crosses its bound', () => {
    const field = (left: number) => (
      <>
        <Textarea aria-label="메모" defaultValue="한글 🦀" aria-describedby="memo-count" />
        <FieldCount id="memo-count" left={left} />
      </>
    )
    const view = render(field(1))
    const textarea = screen.getByRole('textbox', { name: '메모' })
    expect(textarea).toHaveAccessibleDescription('1자 남음')
    view.rerender(field(0))
    expect(textarea).toHaveAccessibleDescription('0자 남음')
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    view.rerender(field(-1))
    expect(screen.getByRole('textbox', { name: '메모' })).toBe(textarea)
    expect(textarea).toHaveValue('한글 🦀')
    expect(textarea).toHaveAccessibleDescription('1자 초과')
    expect(screen.getByRole('status')).toHaveAttribute('id', 'memo-count')
  })

  it.each([-1, 1])('keeps id optional and the default spacing at %i remaining', (left) => {
    const { container } = render(<FieldCount left={left} />)
    expect(container.firstElementChild).not.toHaveAttribute('id')
    expect(container.firstElementChild).toHaveClass('mt-2')
  })

  it.each([
    [-1, ''],
    [1, ''],
    [-1, 'mt-4'],
    [1, 'mt-4'],
  ] as const)('preserves an explicit spacing override at %i remaining: %s', (left, className) => {
    const { container } = render(<FieldCount left={left} className={className} />)
    expect(container.firstElementChild).not.toHaveClass('mt-2')
    if (className) expect(container.firstElementChild).toHaveClass(className)
  })
})
