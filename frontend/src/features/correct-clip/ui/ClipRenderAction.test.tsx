import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { ClipRenderAction } from './ClipRenderAction'

afterEach(cleanup)

async function open(name: string) {
  await userEvent.click(screen.getByRole('button', { name }))
  return within(await screen.findByRole('dialog', { name }))
}

it('offers the browser first on a new project and renders the kind the owner picks', async () => {
  const onRender = vi.fn()
  render(<ClipRenderAction browserAvailable pending={false} disabled={false} onRender={onRender} />)
  // ONE trigger, naming no kind: where the render runs is chosen per render (CLIP-153).
  expect(screen.getAllByRole('button')).toHaveLength(1)
  const choice = await open('렌더하기')
  expect(choice.getAllByRole('button', { name: /에서 렌더$/ }).map((b) => b.textContent)).toEqual([
    '브라우저에서 렌더',
    '서버에서 렌더',
  ])
  expect(onRender).not.toHaveBeenCalled()
  await userEvent.click(choice.getByRole('button', { name: '서버에서 렌더' }))
  expect(onRender).toHaveBeenCalledExactlyOnceWith('server')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('leads with the last successful kind and reads 다시 렌더 only for a render of the current plan', async () => {
  const props = {
    lastKind: 'server' as const,
    browserAvailable: true,
    pending: false,
    disabled: false,
    onRender: vi.fn(),
  }
  const view = render(<ClipRenderAction {...props} currentRender />)
  const choice = await open('다시 렌더')
  expect(choice.getAllByRole('button', { name: /에서 렌더$/ }).map((b) => b.textContent)).toEqual([
    '서버에서 렌더',
    '브라우저에서 렌더',
  ])
  await userEvent.keyboard('{Escape}')
  view.rerender(<ClipRenderAction {...props} />)
  expect(screen.getByRole('button', { name: '렌더하기' })).toBeEnabled()
})

it('keeps the browser option, refused with its reason, while browser rendering is unavailable', async () => {
  const onRender = vi.fn()
  render(
    <ClipRenderAction
      lastKind="browser"
      browserRefusal="capability"
      pending={false}
      disabled={false}
      onRender={onRender}
    />,
  )
  const choice = await open('렌더하기')
  expect(choice.getByRole('button', { name: '브라우저에서 렌더' })).toBeDisabled()
  expect(choice.getByRole('status')).toHaveTextContent(
    /이 브라우저는 필요한 영상·음성 인코딩을 지원하지/,
  )
  await userEvent.click(choice.getByRole('button', { name: '서버에서 렌더' }))
  expect(onRender).toHaveBeenCalledExactlyOnceWith('server')
})

it('is one refused trigger while the plan cannot be rendered, and waits on the render it started', () => {
  const view = render(<ClipRenderAction pending={false} disabled onRender={vi.fn()} />)
  expect(screen.getByRole('button', { name: '렌더하기' })).toBeDisabled()
  view.rerender(<ClipRenderAction pending disabled={false} onRender={vi.fn()} />)
  expect(screen.getByRole('button', { name: '렌더하기' })).toHaveAttribute('aria-busy', 'true')
  view.rerender(
    <ClipRenderAction pending={false} disabled={false} variant="cta" onRender={vi.fn()} />,
  )
  expect(screen.getByRole('button', { name: '렌더하기' })).toHaveClass('bg-button-cta-bg')
})
