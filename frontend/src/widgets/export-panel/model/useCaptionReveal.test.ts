import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useCaptionReveal } from './useCaptionReveal'

/** The hook with its two refs attached the way `CaptionCopy` attaches them: the caption's own
 *  control, and the field a refused copy reveals. */
function setup() {
  const view = renderHook(({ fellBack }: { fellBack: boolean }) => useCaptionReveal(fellBack), {
    initialProps: { fellBack: false },
  })
  const control = document.createElement('button')
  const field = document.createElement('input')
  document.body.append(control, field)
  view.result.current.controlRef.current = control
  view.result.current.fieldRef.current = field
  return { ...view, control, field }
}

afterEach(() => {
  vi.restoreAllMocks()
  document.body.replaceChildren()
})

describe("a caption's fallback field (EXPORT-24)", () => {
  it('is focused and selected once its copy falls back', () => {
    const { rerender, field } = setup()
    const select = vi.spyOn(field, 'select')
    expect(select).not.toHaveBeenCalled()

    rerender({ fellBack: true })
    expect(field).toHaveFocus()
    expect(select).toHaveBeenCalledOnce()
  })

  it('hands the focus back to the caption when it dissolves under the user', () => {
    const { rerender, control, field } = setup()
    rerender({ fellBack: true })

    // A content change unmounts the focused field, which drops the keyboard onto <body>.
    field.remove()
    expect(document.activeElement).toBe(document.body)
    rerender({ fellBack: false })
    expect(control).toHaveFocus()
  })

  it('leaves the focus where the user put it', () => {
    const { rerender, control } = setup()
    rerender({ fellBack: true })
    const elsewhere = document.createElement('button')
    document.body.append(elsewhere)
    elsewhere.focus()

    rerender({ fellBack: false })
    expect(elsewhere).toHaveFocus()
    expect(control).not.toHaveFocus()
  })

  it('touches nothing for a caption that never fell back', () => {
    const { rerender, control, field } = setup()
    rerender({ fellBack: false })
    expect(field).not.toHaveFocus()
    expect(control).not.toHaveFocus()
  })
})
