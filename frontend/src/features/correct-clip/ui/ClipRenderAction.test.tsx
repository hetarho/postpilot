import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { ClipRenderAction } from './ClipRenderAction'

afterEach(cleanup)

it('offers browser on a new project and switches kind only on an explicit choice', async () => {
  const onRender = vi.fn()
  render(<ClipRenderAction browserAvailable pending={false} disabled={false} onRender={onRender} />)
  expect(screen.getByRole('button', { name: '렌더하기 · 브라우저' })).toBeEnabled()
  await userEvent.click(screen.getByRole('button', { name: '서버로 변경' }))
  expect(onRender).not.toHaveBeenCalled()
  await userEvent.click(screen.getByRole('button', { name: '렌더하기 · 서버' }))
  expect(onRender).toHaveBeenCalledExactlyOnceWith('server')
  expect(screen.getByRole('button', { name: '브라우저로 변경' })).toBeEnabled()
})

it('names the last successful kind and distinguishes the current plan from an older render', () => {
  const props = {
    lastKind: 'server' as const,
    browserAvailable: true,
    pending: false,
    disabled: false,
    onRender: vi.fn(),
  }
  const view = render(<ClipRenderAction {...props} currentRender />)
  expect(screen.getByRole('button', { name: '다시 렌더 · 서버' })).toBeEnabled()
  view.rerender(<ClipRenderAction {...props} />)
  expect(screen.getByRole('button', { name: '렌더하기 · 서버' })).toBeEnabled()
})

it('falls back to server with no browser offer while browser rendering is unavailable', async () => {
  const onRender = vi.fn()
  render(
    <ClipRenderAction lastKind="browser" pending={false} disabled={false} onRender={onRender} />,
  )
  expect(screen.getAllByRole('button')).toHaveLength(1)
  await userEvent.click(screen.getByRole('button', { name: '렌더하기 · 서버' }))
  expect(onRender).toHaveBeenCalledExactlyOnceWith('server')
})
