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
  render(
    <ClipRenderAction
      serverPlan="max"
      serverEntitled
      serverWindow={{
        coverageId: 'max',
        startsAt: '',
        endsAt: '',
        allowance: 60,
        used: 0,
        reserved: 0,
        remaining: 60,
      }}
      browserAvailable
      pending={false}
      disabled={false}
      onRender={onRender}
    />,
  )
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

it('offers the supported browser before the last successful kind and reads 다시 렌더 only for a render of the current plan', async () => {
  const props = {
    lastKind: 'server' as const,
    serverPlan: 'max' as const,
    serverEntitled: true,
    browserAvailable: true,
    pending: false,
    disabled: false,
    onRender: vi.fn(),
  }
  const view = render(<ClipRenderAction {...props} currentRender />)
  const choice = await open('다시 렌더')
  expect(choice.getAllByRole('button', { name: /에서 렌더$/ }).map((b) => b.textContent)).toEqual([
    '브라우저에서 렌더',
    '서버에서 렌더',
  ])
  await userEvent.keyboard('{Escape}')
  view.rerender(<ClipRenderAction {...props} />)
  expect(screen.getByRole('button', { name: '렌더하기' })).toBeEnabled()
})

it('keeps the browser option, refused with its reason, while browser rendering is unavailable', async () => {
  const onRender = vi.fn()
  render(
    <ClipRenderAction
      serverPlan="master"
      serverEntitled
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

it('shows the monthly server balance and keeps browser export available when slots are exhausted', async () => {
  const onRender = vi.fn()
  render(
    <ClipRenderAction
      browserAvailable
      serverPlan="max"
      serverEntitled
      serverWindow={{
        coverageId: 'paid',
        startsAt: '2026-09-01T00:00:00Z',
        endsAt: '2026-10-01T00:00:00Z',
        allowance: 6,
        used: 5,
        reserved: 1,
        remaining: 0,
      }}
      pending={false}
      disabled={false}
      onRender={onRender}
    />,
  )
  const choice = await open('렌더하기')
  expect(choice.getByRole('button', { name: '서버에서 렌더' })).toBeDisabled()
  expect(choice.getByText(/사용 5, 예약 1, 남음 0\/6/)).toBeInTheDocument()
  expect(choice.queryByRole('link', { name: '요금제 보기' })).not.toBeInTheDocument()
  expect(choice.getByText(/갱신돼요/)).toBeInTheDocument()
  await userEvent.click(choice.getByRole('button', { name: '브라우저에서 렌더' }))
  expect(onRender).toHaveBeenCalledExactlyOnceWith('browser')
})

it.each(['free', 'light', 'basic', 'pro'] as const)(
  'refuses new server work for %s despite old positive balances',
  async (serverPlan) => {
    const onRender = vi.fn()
    render(
      <ClipRenderAction
        browserAvailable
        serverPlan={serverPlan}
        serverWindow={{
          coverageId: 'old',
          startsAt: '',
          endsAt: '',
          allowance: 6,
          used: 0,
          reserved: 0,
          remaining: 6,
        }}
        pending={false}
        disabled={false}
        onRender={onRender}
      />,
    )
    const choice = await open('렌더하기')
    expect(choice.getByRole('button', { name: '서버에서 렌더' })).toBeDisabled()
    expect(choice.getByText(/새 서버 렌더링은 Max/)).toBeInTheDocument()
    expect(choice.getByRole('link', { name: '요금제 보기' })).toHaveAttribute('href', '/plans')
    await userEvent.click(choice.getByRole('button', { name: '서버에서 렌더' }))
    expect(onRender).not.toHaveBeenCalled()
    await userEvent.click(choice.getByRole('button', { name: '브라우저에서 렌더' }))
    expect(onRender).toHaveBeenCalledExactlyOnceWith('browser')
  },
)

it('keeps an unchanged existing server file reusable after a downgrade', async () => {
  const onRender = vi.fn()
  render(
    <ClipRenderAction
      lastKind="server"
      currentRender
      serverPlan="basic"
      browserAvailable
      pending={false}
      disabled={false}
      onRender={onRender}
    />,
  )
  const choice = await open('다시 렌더')
  expect(choice.getByText(/현재 저장된 서버 결과/)).toBeInTheDocument()
  await userEvent.click(choice.getByRole('button', { name: '서버에서 렌더' }))
  expect(onRender).toHaveBeenCalledExactlyOnceWith('server')
})

it('keeps unknown rights closed without automatically starting either executor', async () => {
  const onRender = vi.fn()
  render(
    <ClipRenderAction
      browserRefusal="capability"
      pending={false}
      disabled={false}
      onRender={onRender}
    />,
  )
  const choice = await open('렌더하기')
  expect(choice.getByRole('button', { name: '서버에서 렌더' })).toBeDisabled()
  expect(choice.getByRole('button', { name: '브라우저에서 렌더' })).toBeDisabled()
  expect(choice.getByText(/현재 플랜을 확인한 뒤/)).toBeInTheDocument()
  expect(onRender).not.toHaveBeenCalled()
})
