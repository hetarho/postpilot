import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { SegmentedControl } from '../segmented-control/SegmentedControl'
import { WorkspaceHeader } from './WorkspaceHeader'

const destination = { href: '/naver-blog?q=산책', label: '네이버 블로그로 돌아가기' }

// Observe the React handler before suppressing jsdom's unsupported document navigation.
function click(link: HTMLElement, init: MouseEventInit = {}, alreadyPrevented = false) {
  const event = new MouseEvent('click', { bubbles: true, cancelable: true, button: 0, ...init })
  if (alreadyPrevented) event.preventDefault()
  let preventedByHandler = false
  document.addEventListener(
    'click',
    (observed) => {
      preventedByHandler = observed.defaultPrevented
      observed.preventDefault()
    },
    { once: true },
  )
  fireEvent(link, event)
  return preventedByHandler
}

it('names its one icon return with its actual destination and delegates normal clicks once', () => {
  const onReturn = vi.fn()
  render(<WorkspaceHeader returnTo={{ ...destination, onReturn }} />)
  const link = screen.getByRole('link', { name: destination.label })
  expect(screen.getAllByRole('link')).toHaveLength(1)
  expect(link).toHaveAttribute('href', destination.href)
  expect(link.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  expect(screen.getByText(destination.label)).toHaveClass('sr-only')
  expect(click(link)).toBe(true)
  expect(onReturn).toHaveBeenCalledExactlyOnceWith()
})

it.each([
  { metaKey: true },
  { ctrlKey: true },
  { shiftKey: true },
  { altKey: true },
  { button: 1 },
  { button: 2 },
])('leaves native link behavior and owner work untouched for %j', (init) => {
  const onReturn = vi.fn()
  render(<WorkspaceHeader returnTo={{ ...destination, onReturn }} />)
  expect(click(screen.getByRole('link'), init)).toBe(false)
  expect(onReturn).not.toHaveBeenCalled()
})

it('respects a previously prevented click and keeps a callback-free link native', () => {
  const onReturn = vi.fn()
  const view = render(<WorkspaceHeader returnTo={{ ...destination, onReturn }} />)
  click(screen.getByRole('link'), {}, true)
  expect(onReturn).not.toHaveBeenCalled()
  view.rerender(<WorkspaceHeader returnTo={destination} />)
  expect(click(screen.getByRole('link'))).toBe(false)
})

it('keeps the lifecycle and pre-mounted live region while context, actions and long status change', () => {
  const onChange = vi.fn()
  const steps = (
    <SegmentedControl
      value="generate"
      options={[
        { value: 'generate', label: '글 생성' },
        { value: 'refine', label: '글 다듬기' },
        { value: 'finish', label: '글 완성' },
      ]}
      onChange={onChange}
      controls="post-step-panel"
      ariaLabel="글 단계"
      variant="steps"
    />
  )
  const status = (text: string) => (
    <p role="status" aria-live="polite" className="order-last w-full break-words whitespace-normal">
      {text}
    </p>
  )
  const view = render(<WorkspaceHeader returnTo={destination} steps={steps} status={status('')} />)
  const tabs = screen.getAllByRole('tab')
  const live = screen.getByRole('status')
  tabs[0].focus()
  const refusal =
    '저장된 수정본이 변경되었어요. 작성 중인 내용은 유지한 채 최신 버전을 확인한 뒤 다시 저장해 주세요.'
  view.rerender(
    <WorkspaceHeader
      returnTo={destination}
      contextTitle="계속 편집하던 기록"
      steps={steps}
      actions={<button type="button">삭제</button>}
      status={status(refusal)}
    />,
  )
  expect(screen.getByText('계속 편집하던 기록')).toBeVisible()
  expect(screen.getByRole('status')).toBe(live)
  expect(live).toHaveTextContent(refusal)
  expect(screen.getAllByRole('tab')[0]).toBe(tabs[0])
  expect(tabs[0]).toHaveFocus()
  expect(tabs[0]).toHaveAttribute('aria-controls', 'post-step-panel')
  fireEvent.keyDown(screen.getByRole('tablist'), { key: 'ArrowRight' })
  expect(onChange).toHaveBeenCalledWith('refine')
})
