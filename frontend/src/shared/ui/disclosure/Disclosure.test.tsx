import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { Disclosure } from './Disclosure'

describe('Disclosure', () => {
  it('opens and closes the region under its heading, uncontrolled', async () => {
    const user = userEvent.setup()
    render(
      <Disclosure title="스토리라인">
        <p>첫 문단</p>
      </Disclosure>,
    )
    const button = screen.getByRole('button', { name: '스토리라인' })
    expect(button).toHaveAttribute('aria-expanded', 'false')
    expect(button).not.toHaveAttribute('aria-controls')
    expect(screen.queryByText('첫 문단')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: '스토리라인' })).toContainElement(button)

    await user.click(button)
    expect(button).toHaveAttribute('aria-expanded', 'true')
    const region = screen.getByRole('region', { name: '스토리라인' })
    expect(button).toHaveAttribute('aria-controls', region.id)
    expect(region).toHaveTextContent('첫 문단')

    await user.click(button)
    expect(screen.queryByRole('region')).not.toBeInTheDocument()
  })

  it('reads as a list entry, not a section heading, at the row size', () => {
    render(
      <Disclosure title="재료에 있는 사실만" size="row" headingLevel={3}>
        <p>본문</p>
      </Disclosure>,
    )
    const button = screen.getByRole('button', { name: '재료에 있는 사실만' })
    expect(button).toHaveClass('text-sm', 'font-medium', 'min-h-11')
    expect(button).not.toHaveClass('text-lg')
    expect(screen.getByRole('heading', { level: 3 })).toContainElement(button)
  })

  it('starts open from defaultOpen and reports every toggle', async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    render(
      <Disclosure title="스토리라인" defaultOpen onOpenChange={onOpenChange} headingLevel={3}>
        <p>첫 문단</p>
      </Disclosure>,
    )
    expect(screen.getByRole('region', { name: '스토리라인' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '스토리라인' }))
    expect(onOpenChange).toHaveBeenLastCalledWith(false)
    expect(screen.getByRole('heading', { level: 3 })).toBeInTheDocument()
  })

  it('follows its owner when controlled', async () => {
    const user = userEvent.setup()
    function Controlled() {
      const [open, setOpen] = useState(true)
      return (
        <>
          <Disclosure title="스토리라인" open={open} onOpenChange={setOpen}>
            <p>첫 문단</p>
          </Disclosure>
          <button type="button" onClick={() => setOpen(true)}>
            열기
          </button>
        </>
      )
    }
    render(<Controlled />)
    await user.click(screen.getByRole('button', { name: '스토리라인' }))
    expect(screen.queryByText('첫 문단')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '열기' }))
    expect(screen.getByText('첫 문단')).toBeInTheDocument()
  })
})
