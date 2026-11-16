import { useState } from 'react'
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { SM_MEDIA_QUERY } from '../media-query/useMediaQuery'
import { Dialog } from '../dialog/Dialog'
import { Listbox } from '../listbox/Listbox'
import { Toggletip } from '../toggletip/Toggletip'
import { Popover } from './Popover'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function layout(anchor: () => DOMRect) {
  vi.stubGlobal('innerWidth', 1000)
  vi.stubGlobal('innerHeight', 720)
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (
    this: HTMLElement,
  ) {
    return this.getAttribute('role') === 'dialog' ? new DOMRect(0, 0, 288, 500) : anchor()
  })
  vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(500)
}

it('opens from the keyboard, closes on Escape, and restores trigger focus', async () => {
  const user = userEvent.setup()
  render(<Popover label="테스트 옵션">{() => <input aria-label="옵션 값" />}</Popover>)
  const trigger = screen.getByRole('button', { name: '테스트 옵션' })
  trigger.focus()
  await user.keyboard('{Enter}')
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toBeInTheDocument()
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toHaveClass('fixed')
  expect(screen.getByLabelText('옵션 값')).toHaveFocus()
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(trigger).toHaveFocus()
  expect(trigger).toHaveClass('min-h-10', 'pointer-coarse:min-h-11')
})

it('bounds a below-header panel to the viewport and lets its contents scroll', async () => {
  const user = userEvent.setup()
  render(
    <Popover label="계정" placement="below">
      {() => <button>로그아웃</button>}
    </Popover>,
  )

  await user.click(screen.getByRole('button', { name: '계정' }))
  expect(screen.getByRole('dialog', { name: '계정' })).toHaveClass(
    'overflow-y-auto',
    'overscroll-contain',
  )
})

it('dismisses when the user presses outside', async () => {
  const user = userEvent.setup()
  render(
    <div>
      <Popover label="테스트 옵션">{() => <span>내용</span>}</Popover>
      <button>바깥</button>
    </div>,
  )
  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  await user.click(screen.getByRole('button', { name: '바깥' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

it('cycles Tab focus inside the open dialog', async () => {
  const user = userEvent.setup()
  render(
    <div>
      <Popover label="테스트 옵션">
        {() => (
          <>
            <input aria-label="첫 번째 옵션" />
            <button>마지막 옵션</button>
          </>
        )}
      </Popover>
      <button>바깥</button>
    </div>,
  )

  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  const first = screen.getByRole('textbox', { name: '첫 번째 옵션' })
  const last = screen.getByRole('button', { name: '마지막 옵션' })
  expect(first).toHaveFocus()

  await user.tab({ shift: true })
  expect(last).toHaveFocus()
  await user.tab()
  expect(first).toHaveFocus()
  expect(screen.getByRole('button', { name: '바깥' })).not.toHaveFocus()
})

it('includes a native details summary in the focus cycle', async () => {
  const user = userEvent.setup()
  render(
    <Popover label="오류 세부정보">
      {() => (
        <>
          <button>다시 시도</button>
          <details>
            <summary>기술 세부 정보</summary>
            실패 코드
          </details>
        </>
      )}
    </Popover>,
  )

  await user.click(screen.getByRole('button', { name: '오류 세부정보' }))
  expect(screen.getByRole('button', { name: '다시 시도' })).toHaveFocus()
  await user.tab()
  expect(screen.getByText('기술 세부 정보')).toHaveFocus()
  await user.tab()
  expect(screen.getByRole('button', { name: '다시 시도' })).toHaveFocus()
})

it('keeps the right-aligned panel and clamps its width to the viewport by default', async () => {
  const user = userEvent.setup()
  layout(() => new DOMRect(700, 600, 100, 44))
  render(<Popover label="테스트 옵션">{() => <span>내용</span>}</Popover>)

  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  const panel = screen.getByRole('dialog', { name: '테스트 옵션' })
  expect(panel).toHaveClass('max-w-popover')
  expect(panel.style.left).toBe('512px')
})

it('pins the panel to the trigger start edge when asked', async () => {
  const user = userEvent.setup()
  layout(() => new DOMRect(300, 600, 100, 44))
  render(
    <Popover label="테스트 옵션" align="start">
      {() => <span>내용</span>}
    </Popover>,
  )

  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  const panel = screen.getByRole('dialog', { name: '테스트 옵션' })
  expect(panel.style.left).toBe('300px')
})

it('opens as a bottom sheet below sm: when phone="sheet", with a visible way out', async () => {
  const user = userEvent.setup()
  render(
    <Popover label="글쓰기 옵션" phone="sheet">
      {(close) => (
        <button type="button" onClick={close}>
          저장
        </button>
      )}
    </Popover>,
  )

  await user.click(screen.getByRole('button', { name: '글쓰기 옵션' }))
  const sheet = screen.getByRole('dialog', { name: '글쓰기 옵션' })
  expect(sheet).toHaveClass('rounded-t-xl', 'pb-sheet-b')
  expect(document.body.style.overflow).toBe('hidden')

  await user.click(screen.getByRole('button', { name: '닫기' }))
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(document.body.style.overflow).toBe('')
})

it('stays a popover from sm: up even when phone="sheet"', async () => {
  const user = userEvent.setup()
  const original = window.matchMedia
  window.matchMedia = ((query: string) => ({
    matches: query === SM_MEDIA_QUERY,
    media: query,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  })) as typeof window.matchMedia
  try {
    render(
      <Popover label="글쓰기 옵션" phone="sheet">
        {() => <button>저장</button>}
      </Popover>,
    )
    await user.click(screen.getByRole('button', { name: '글쓰기 옵션' }))
    expect(screen.getByRole('dialog', { name: '글쓰기 옵션' })).toHaveClass('fixed')
    expect(document.body.style.overflow).toBe('')
  } finally {
    window.matchMedia = original
  }
})

it('survives a modal opened from inside its own panel', async () => {
  const user = userEvent.setup()
  function Harness() {
    const [open, setOpen] = useState(false)
    return (
      <Popover label="테스트 옵션">
        {() => (
          <>
            <button onClick={() => setOpen(true)}>확인 열기</button>
            <Dialog
              open={open}
              title="정말 바꿀까요?"
              confirmLabel="바꾸기"
              onConfirm={() => setOpen(false)}
              onClose={() => setOpen(false)}
            >
              설명
            </Dialog>
          </>
        )}
      </Popover>
    )
  }
  render(<Harness />)

  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  await user.click(screen.getByRole('button', { name: '확인 열기' }))
  await user.click(screen.getByRole('button', { name: '바꾸기' }))

  // The panel is still open, so the control that opened the dialog is still mounted.
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: '확인 열기' }))
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog', { name: '정말 바꿀까요?' })).toBeNull()
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toBeInTheDocument()
})

// `Toggletip` portals its panel to the body, as a listbox does, so a press inside one opened from
// this panel lands outside its root. It must still count as inside.
it('survives a press inside an anchored panel opened from its own panel', async () => {
  const user = userEvent.setup()
  render(
    <Popover label="테스트 옵션">
      {() => (
        <p>
          <Toggletip label="반복 줄이기 설명">가장 많이 쓴 명사의 비율이에요.</Toggletip>
        </p>
      )}
    </Popover>,
  )
  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))

  await user.click(screen.getByRole('button', { name: '반복 줄이기 설명' }))
  const bubble = document.body.querySelector<HTMLElement>(
    '[data-anchored-panel][aria-hidden="true"]',
  )
  expect(bubble).not.toBeNull()
  await user.pointer({ keys: '[MouseLeft]', target: bubble! })
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toBeInTheDocument()
})

it('bounds the above panel and lets a tall brief scroll inside it', async () => {
  const user = userEvent.setup()
  render(<Popover label="테스트 옵션">{() => <span>내용</span>}</Popover>)

  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toHaveClass(
    'overflow-y-auto',
    'overscroll-contain',
  )
})

it('portals a long review-row brief above the dock and flips it into the available space', async () => {
  const user = userEvent.setup()
  layout(() => new DOMRect(700, 600, 100, 44))
  const { container } = render(
    <div className="overflow-hidden">
      <Popover label="글쓰기 옵션" placement="below">
        {() => <button>저장</button>}
      </Popover>
    </div>,
  )
  await user.click(screen.getByRole('button', { name: '글쓰기 옵션' }))
  const panel = screen.getByRole('dialog', { name: '글쓰기 옵션' })
  expect(container.contains(panel)).toBe(false)
  expect(panel.parentElement?.parentElement).toBe(document.body)
  expect(panel).toHaveClass('fixed', 'z-overlay-panel')
  expect(panel.style.bottom).toBe('128px')
  expect(panel.style.top).toBe('')
  expect(Number.parseFloat(panel.style.maxHeight)).toBeLessThanOrEqual(600 - 8 - 16)
  await user.click(within(panel).getByRole('button', { name: '저장' }))
  expect(panel).toBeInTheDocument()
})

it('flips a preferred-above panel down near the top edge and keeps its left gutter', async () => {
  const user = userEvent.setup()
  layout(() => new DOMRect(4, 20, 100, 44))
  render(<Popover label="옵션">{() => <button>저장</button>}</Popover>)
  await user.click(screen.getByRole('button', { name: '옵션' }))
  const panel = screen.getByRole('dialog')
  expect(panel.style.top).toBe('72px')
  expect(panel.style.bottom).toBe('')
  expect(panel.style.left).toBe('16px')
})

it('tracks scrolling and resize and dismisses once its trigger leaves the viewport', async () => {
  const user = userEvent.setup()
  let anchor = new DOMRect(700, 600, 100, 44)
  layout(() => anchor)
  const closed = vi.fn()
  render(
    <Popover label="옵션" onClose={closed}>
      {() => <button>저장</button>}
    </Popover>,
  )
  await user.click(screen.getByRole('button', { name: '옵션' }))
  anchor = new DOMRect(500, 500, 100, 44)
  fireEvent.scroll(window)
  expect(screen.getByRole('dialog').style.left).toBe('312px')
  vi.stubGlobal('innerWidth', 560)
  fireEvent.resize(window)
  expect(screen.getByRole('dialog').style.left).toBe('256px')
  anchor = new DOMRect(500, -100, 100, 44)
  fireEvent.scroll(window)
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(closed).toHaveBeenCalledTimes(1)
})

it('closes only the innermost portalled popover on Escape', async () => {
  const user = userEvent.setup()
  render(
    <Popover label="바깥 옵션">
      {() => <Popover label="안쪽 옵션">{() => <button>선택</button>}</Popover>}
    </Popover>,
  )
  await user.click(screen.getByRole('button', { name: '바깥 옵션' }))
  await user.click(screen.getByRole('button', { name: '안쪽 옵션' }))
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog', { name: '안쪽 옵션' })).toBeNull()
  expect(screen.getByRole('dialog', { name: '바깥 옵션' })).toBeInTheDocument()
  expect(screen.getByRole('button', { name: '안쪽 옵션' })).toHaveFocus()
})

it('closes only the listbox inside it on the first Escape', async () => {
  const user = userEvent.setup()
  render(
    <Popover label="글쓰기 옵션">
      {() => (
        <Listbox<string>
          aria-label="말투"
          value="a"
          options={[
            { value: 'a', label: '일상' },
            { value: 'b', label: '리뷰' },
          ]}
          onChange={() => undefined}
        />
      )}
    </Popover>,
  )

  await user.click(screen.getByRole('button', { name: '글쓰기 옵션' }))
  await user.click(screen.getByRole('combobox', { name: '말투' }))
  expect(screen.getByRole('listbox')).toBeInTheDocument()

  await user.keyboard('{Escape}')
  expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  expect(screen.getByRole('dialog', { name: '글쓰기 옵션' })).toBeInTheDocument()

  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog', { name: '글쓰기 옵션' })).not.toBeInTheDocument()
})
