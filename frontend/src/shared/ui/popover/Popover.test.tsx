import { useState } from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import { SM_MEDIA_QUERY } from './useMediaQuery'
import { Dialog } from '../dialog/Dialog'
import { Listbox } from '../listbox/Listbox'
import { Popover } from './Popover'

it('opens from the keyboard, closes on Escape, and restores trigger focus', async () => {
  const user = userEvent.setup()
  render(<Popover label="테스트 옵션">{() => <input aria-label="옵션 값" />}</Popover>)
  const trigger = screen.getByRole('button', { name: '테스트 옵션' })
  trigger.focus()
  await user.keyboard('{Enter}')
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toBeInTheDocument()
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toHaveClass('bottom-full')
  expect(screen.getByLabelText('옵션 값')).toHaveFocus()
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(trigger).toHaveFocus()
  expect(trigger).toHaveClass('min-h-11')
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
    'max-h-popover-below-max',
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
  render(<Popover label="테스트 옵션">{() => <span>내용</span>}</Popover>)

  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  const panel = screen.getByRole('dialog', { name: '테스트 옵션' })
  expect(panel).toHaveClass('right-0', 'max-w-popover')
  expect(panel).not.toHaveClass('left-0')
})

it('pins the panel to the trigger start edge when asked', async () => {
  const user = userEvent.setup()
  render(
    <Popover label="테스트 옵션" align="start">
      {() => <span>내용</span>}
    </Popover>,
  )

  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  const panel = screen.getByRole('dialog', { name: '테스트 옵션' })
  expect(panel).toHaveClass('left-0')
  expect(panel).not.toHaveClass('right-0')
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
  expect(sheet).toHaveClass('rounded-t-xl', 'pb-safe-b')
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
    expect(screen.getByRole('dialog', { name: '글쓰기 옵션' })).toHaveClass('bottom-full')
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
})

it('bounds the above panel and lets a tall brief scroll inside it', async () => {
  const user = userEvent.setup()
  render(<Popover label="테스트 옵션">{() => <span>내용</span>}</Popover>)

  await user.click(screen.getByRole('button', { name: '테스트 옵션' }))
  expect(screen.getByRole('dialog', { name: '테스트 옵션' })).toHaveClass(
    'max-h-popover-above-max',
    'overflow-y-auto',
    'overscroll-contain',
  )
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
