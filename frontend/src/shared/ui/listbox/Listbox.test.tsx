import { useState } from 'react'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { FieldLabel } from '../field/FieldLabel'
import { Sheet } from '../sheet/Sheet'
import { Listbox } from './Listbox'

type Fruit = 'apple' | 'pear' | 'plum'

function Harness({ onChanged }: { onChanged?: (value: Fruit) => void }) {
  const [value, setValue] = useState<Fruit>('pear')
  return (
    <>
      <FieldLabel id="fruit-label" htmlFor="fruit">
        과일
      </FieldLabel>
      <Listbox<Fruit>
        id="fruit"
        aria-labelledby="fruit-label"
        value={value}
        options={[
          { value: 'apple', label: '사과' },
          { value: 'pear', label: '배' },
          { value: 'plum', label: '자두', disabled: true },
        ]}
        onChange={(next) => {
          setValue(next)
          onChanged?.(next)
        }}
      />
    </>
  )
}

afterEach(cleanup)

/** A 44px-tall trigger whose top edge sits `top` pixels down jsdom's 768px viewport. */
function rectAt(top: number): DOMRect {
  return { top, bottom: top + 44, left: 0, right: 200, width: 200, height: 44 } as DOMRect
}

describe('Listbox', () => {
  it('names itself with its label and its current option, and wears the field well', () => {
    render(<Harness />)
    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    expect(trigger).toHaveClass('bg-field-bg', 'min-h-11')
    expect(trigger).toHaveAttribute('aria-haspopup', 'listbox')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('opens an app-drawn panel whose current option holds focus', async () => {
    const user = userEvent.setup()
    render(<Harness />)

    await user.click(screen.getByRole('combobox', { name: '과일 배' }))

    const panel = screen.getByRole('listbox', { name: '과일' })
    // Its own surface, not the one every overlay wears: an open panel inside a sheet used to be
    // exactly the colour of the sheet behind it.
    expect(panel).toHaveClass('bg-surface-overlay', 'z-overlay-panel', 'fixed')
    expect(panel).not.toHaveClass('bg-surface-highest')
    expect(panel).not.toHaveClass('border')
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual([
      '사과',
      '배',
      '자두',
    ])
    expect(screen.getByRole('option', { name: '배', selected: true })).toHaveFocus()
  })

  it('moves with the arrows and Home/End, and selects with Enter', async () => {
    const user = userEvent.setup()
    const onChanged = vi.fn()
    render(<Harness onChanged={onChanged} />)

    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    trigger.focus()
    await user.keyboard('{ArrowDown}')
    expect(screen.getByRole('option', { name: '배' })).toHaveFocus()

    await user.keyboard('{End}')
    expect(screen.getByRole('option', { name: '자두' })).toHaveFocus()
    await user.keyboard('{Home}')
    expect(screen.getByRole('option', { name: '사과' })).toHaveFocus()

    await user.keyboard('{Enter}')
    expect(onChanged).toHaveBeenCalledWith('apple')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: '과일 사과' })).toHaveFocus()
  })

  it('refuses a disabled option without closing over the reason it is listed', async () => {
    const user = userEvent.setup()
    const onChanged = vi.fn()
    render(<Harness onChanged={onChanged} />)

    await user.click(screen.getByRole('combobox', { name: '과일 배' }))
    await user.click(screen.getByRole('option', { name: '자두' }))

    expect(onChanged).not.toHaveBeenCalled()
    expect(screen.getByRole('listbox')).toBeInTheDocument()
  })

  it('closes on Escape and on an outside press, returning focus to the trigger', async () => {
    const user = userEvent.setup()
    render(
      <div>
        <Harness />
        <button>바깥</button>
      </div>,
    )

    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    await user.click(trigger)
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()

    await user.click(trigger)
    await user.click(screen.getByRole('button', { name: '바깥' }))
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('shows the placeholder for a value no option carries, and reports invalid state', () => {
    render(
      <Listbox<string>
        aria-label="모델"
        aria-invalid
        value=""
        placeholder="선택하세요"
        options={[{ value: 'a', label: 'A' }]}
        onChange={() => undefined}
      />,
    )
    const trigger = screen.getByRole('combobox', { name: '모델' })
    expect(trigger).toHaveTextContent('선택하세요')
    expect(trigger).toHaveAttribute('aria-invalid', 'true')
  })

  it('round-trips a numeric value without a stringly-typed cast', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <Listbox<number>
        aria-label="제목 단계"
        value={2}
        options={[
          { value: 2, label: 'H2' },
          { value: 3, label: 'H3' },
        ]}
        onChange={onChange}
      />,
    )
    await user.click(screen.getByRole('combobox', { name: '제목 단계' }))
    await user.click(screen.getByRole('option', { name: 'H3' }))
    expect(onChange).toHaveBeenCalledWith(3)
  })

  // The bug this measurement replaced: a `dvh` ceiling is the same number wherever the trigger
  // sits, so a field in the editor's docked bar opened a panel half the screen tall into the
  // ~0px of room beneath it, and every option was past the bottom edge.
  it('bounds the open panel to the room below the trigger', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    // jsdom has no layout engine, so the anchor is stated rather than measured. 500px down a
    // 768px viewport leaves 224px under the trigger, less the 4px gap and the 16px gutter.
    trigger.getBoundingClientRect = () => rectAt(500)

    await user.click(trigger)
    const panel = screen.getByRole('listbox')
    expect(panel).toHaveClass('overflow-y-auto', 'overscroll-contain')
    expect(panel).toHaveAttribute('data-drop', 'down')
    // 544px + the 4px gap: the panel hangs off the trigger's own bottom edge.
    expect(panel.style.top).toBe('548px')
    expect(panel.style.maxHeight).toBe('204px')
  })

  it('stops at half the viewport where there is more room than a list needs', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    trigger.getBoundingClientRect = () => rectAt(120)

    await user.click(trigger)
    // 604px are free, but a forty-model catalog opening that tall buries the field it belongs to.
    expect(screen.getByRole('listbox').style.maxHeight).toBe('384px')
  })

  it('flips above a trigger with no room under it, and scrolls inside what is there', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    // A docked bar's field: 24px of viewport under it, 700px over it.
    trigger.getBoundingClientRect = () => rectAt(700)

    await user.click(trigger)
    const panel = screen.getByRole('listbox')
    expect(panel).toHaveAttribute('data-drop', 'up')
    // Pinned to the viewport's BOTTOM edge instead: 768 - 700 + the 4px gap.
    expect(panel.style.bottom).toBe('72px')
    expect(panel.style.top).toBe('')
    expect(panel.style.maxHeight).toBe('384px')
  })

  // The bug the portal replaced: as an `absolute` sibling of its trigger the panel was clipped by
  // every scrolling ancestor, and a sheet's body is one — worst for a field near the bottom, whose
  // panel flips upward and out of that scroller entirely.
  describe('inside a sheet', () => {
    function SheetHarness({ onClose = () => undefined }: { onClose?: () => void }) {
      return (
        <Sheet open label="새 말투" onClose={onClose}>
          <Harness />
          <button>시트 안 다음 컨트롤</button>
        </Sheet>
      )
    }

    it('renders its panel outside the sheet, with every option reachable', async () => {
      const user = userEvent.setup()
      render(<SheetHarness />)

      await user.click(screen.getByRole('combobox', { name: '과일 배' }))

      const panel = screen.getByRole('listbox', { name: '과일' })
      expect(panel.parentElement).toBe(document.body)
      expect(panel.closest('[role="dialog"]')).toBeNull()
      expect(screen.getAllByRole('option')).toHaveLength(3)
    })

    it('lets one Escape dismiss the listbox and leave the sheet open', async () => {
      const user = userEvent.setup()
      const onClose = vi.fn()
      render(<SheetHarness onClose={onClose} />)

      const trigger = screen.getByRole('combobox', { name: '과일 배' })
      await user.click(trigger)
      await user.keyboard('{Escape}')

      expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
      expect(onClose).not.toHaveBeenCalled()
      expect(trigger).toHaveFocus()
      expect(screen.getByRole('dialog', { name: '새 말투' })).toBeInTheDocument()
    })

    it('hands Tab back to the field, so the traversal continues inside the sheet', async () => {
      const user = userEvent.setup()
      render(<SheetHarness />)

      await user.click(screen.getByRole('combobox', { name: '과일 배' }))
      await user.keyboard('{Tab}')

      expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: '시트 안 다음 컨트롤' })).toHaveFocus()
    })
  })

  it('closes rather than detaching once a scroll carries its trigger off screen', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    const trigger = screen.getByRole('combobox', { name: '과일 배' })
    trigger.getBoundingClientRect = () => rectAt(300)
    await user.click(trigger)
    expect(screen.getByRole('listbox')).toBeInTheDocument()

    trigger.getBoundingClientRect = () => rectAt(-80)
    window.dispatchEvent(new Event('scroll'))

    await waitFor(() => expect(screen.queryByRole('listbox')).not.toBeInTheDocument())
    // Focus was on an option that has just been unmounted. It goes back to the trigger rather
    // than to the document — losing it would let the next Tab escape a sheet's focus trap.
    await waitFor(() => expect(trigger).toHaveFocus())
  })

  it('does not open while disabled', async () => {
    const user = userEvent.setup()
    render(
      <Listbox<string>
        aria-label="모델"
        disabled
        value="a"
        options={[{ value: 'a', label: 'A' }]}
        onChange={() => undefined}
      />,
    )
    await user.click(screen.getByRole('combobox', { name: '모델' }))
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })
})
