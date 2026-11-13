import { createRef } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { SurfaceButton } from './SurfaceButton'
import { MediaAction } from '../media-action/MediaAction'

it.each(['card', 'row', 'content'] as const)(
  'forwards native semantics, content, ref and disabled behavior for %s',
  async (presentation) => {
    const user = userEvent.setup()
    const ref = createRef<HTMLButtonElement>()
    const onClick = vi.fn()
    const { rerender } = render(
      <SurfaceButton presentation={presentation} ref={ref} aria-expanded="false" onClick={onClick}>
        <span>Open item</span>
      </SurfaceButton>,
    )
    const button = screen.getByRole('button', { name: 'Open item' })
    expect(ref.current).toBe(button)
    expect(button).toHaveAttribute('type', 'button')
    expect(button).toHaveAttribute('aria-expanded', 'false')
    await user.click(button)
    expect(onClick).toHaveBeenCalledOnce()
    rerender(
      <SurfaceButton presentation={presentation} type="submit" disabled onClick={onClick}>
        Open item
      </SurfaceButton>,
    )
    expect(button).toHaveAttribute('type', 'submit')
    await user.click(button)
    expect(onClick).toHaveBeenCalledOnce()
  },
)

it('forwards a named media action, ref, children and disabled state', async () => {
  const user = userEvent.setup()
  const ref = createRef<HTMLButtonElement>()
  const onClick = vi.fn()
  const { rerender } = render(
    <MediaAction ref={ref} aria-label="Play preview" onClick={onClick}>
      <span>Preview</span>
    </MediaAction>,
  )
  const action = screen.getByRole('button', { name: 'Play preview' })
  expect(ref.current).toBe(action)
  expect(action).toHaveTextContent('Preview')
  await user.tab()
  expect(action).toHaveFocus()
  await user.keyboard('{Enter}')
  expect(onClick).toHaveBeenCalledOnce()
  rerender(<MediaAction aria-label="Play preview" disabled onClick={onClick} />)
  await user.click(action)
  expect(onClick).toHaveBeenCalledOnce()
})
