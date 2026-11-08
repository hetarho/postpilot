import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Editable } from './Editable'

/** A field in the shape every caller gives it: the edit view swaps the pencil out for its own
 *  controls, so the focused button leaves the document in both directions. `saved` stands for a
 *  save that settles later; 저장 exits once it does. */
function renderField({
  autoFocus = false,
  saved = Promise.resolve(),
}: { autoFocus?: boolean; saved?: Promise<void> } = {}) {
  render(
    <>
      <input aria-label="다른 칸" />
      <Editable
        editLabel="이름 수정"
        edit={(exit) => (
          <>
            <button>종류</button>
            <input aria-label="이름" autoFocus={autoFocus} />
            <button onClick={() => void saved.then(exit)}>저장</button>
          </>
        )}
      >
        <p>하람</p>
      </Editable>
    </>,
  )
  return { pencil: () => screen.getByRole('button', { name: '이름 수정' }) }
}

describe('Editable keyboard focus', () => {
  it('moves focus into an edit view that places none, when the pencil is pressed from the keyboard', async () => {
    const user = userEvent.setup()
    const { pencil } = renderField()

    pencil().focus()
    await user.keyboard('{Enter}')

    expect(screen.getByRole('button', { name: '종류' })).toHaveFocus()
  })

  it('leaves an autoFocus field where it put focus', async () => {
    const user = userEvent.setup()
    const { pencil } = renderField({ autoFocus: true })

    pencil().focus()
    await user.keyboard('{Enter}')

    expect(screen.getByLabelText('이름')).toHaveFocus()
  })

  // A tap must not focus a field: that would raise the phone keyboard over the card it opened.
  it('does not move focus on a pointer press', async () => {
    const user = userEvent.setup()
    const { pencil } = renderField()

    await user.click(pencil())

    expect(screen.getByRole('button', { name: '종류' })).not.toHaveFocus()
    expect(screen.getByLabelText('이름')).not.toHaveFocus()
  })

  it('returns focus to the pencil when a keyboard exit leaves edit mode', async () => {
    const user = userEvent.setup()
    const { pencil } = renderField()
    await user.click(pencil())

    screen.getByRole('button', { name: '저장' }).focus()
    const focus = vi.spyOn(HTMLElement.prototype, 'focus')
    await user.keyboard('{Enter}')

    expect(pencil()).toHaveFocus()
    // A tall edit card collapsing must not scroll the page to the pencil.
    expect(focus).toHaveBeenCalledWith({ preventScroll: true })
    focus.mockRestore()
  })

  it('does not take focus back when it already moved elsewhere', async () => {
    const user = userEvent.setup()
    let settle = () => {}
    const saved = new Promise<void>((resolve) => {
      settle = resolve
    })
    const { pencil } = renderField({ saved })
    await user.click(pencil())

    screen.getByRole('button', { name: '저장' }).focus()
    await user.keyboard('{Enter}')
    screen.getByLabelText('다른 칸').focus()
    await act(async () => settle())

    expect(pencil()).toBeInTheDocument()
    expect(screen.getByLabelText('다른 칸')).toHaveFocus()
  })
})
