import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import type { DefaultGuideline } from '../model/types'
import { DefaultGuidelineRow } from './DefaultGuidelineRow'

afterEach(() => cleanup())

const NAMING: DefaultGuideline = {
  key: 'naming',
  enabled: true,
  name: '메모의 이름으로',
  text: '가게와 메뉴는 메모에 적힌 이름으로 쓰세요.',
  koreanTargetOnly: false,
  memoriesOnly: false,
}

function renderRow(guideline: DefaultGuideline, props: { errorMessage?: string } = {}) {
  const onToggle = vi.fn()
  render(
    <ul>
      <DefaultGuidelineRow guideline={guideline} onToggle={onToggle} {...props} />
    </ul>,
  )
  return onToggle
}

// GUIDE-19: the row shows the server's name and text word for word, marks it 추천, and names its
// switch after the guideline so a screen reader can tell eleven switches apart.
it('shows the name, the text, 추천 and a switch named after the guideline', async () => {
  const user = userEvent.setup()
  const onToggle = renderRow(NAMING)

  expect(screen.getByText('메모의 이름으로')).toBeInTheDocument()
  expect(screen.getByText('가게와 메뉴는 메모에 적힌 이름으로 쓰세요.')).toBeInTheDocument()
  expect(screen.getByText('추천')).toBeInTheDocument()
  const control = screen.getByRole('switch', { name: '메모의 이름으로 사용' })
  expect(control).toBeChecked()
  expect(screen.queryByText('한국어 글에만 적용돼요')).not.toBeInTheDocument()
  expect(screen.queryByText('기억 사용을 켠 글에만 적용돼요')).not.toBeInTheDocument()

  await user.click(control)
  expect(onToggle).toHaveBeenCalledWith(false)
})

// GUIDE-43: a switched-off row is still listed, dimmed, and flipping it asks to turn it on.
it('keeps a switched-off row, dimmed, one flip from on', async () => {
  const user = userEvent.setup()
  const onToggle = renderRow({ ...NAMING, enabled: false })

  const control = screen.getByRole('switch', { name: '메모의 이름으로 사용' })
  expect(control).not.toBeChecked()
  expect(screen.getByText('메모의 이름으로').closest('.opacity-60')).not.toBeNull()
  await user.click(control)
  expect(onToggle).toHaveBeenCalledWith(true)
})

it('says a Korean-only default applies only to a Korean post, and shows a refusal', () => {
  renderRow(
    { ...NAMING, key: 'natural_korean', name: '자연스러운 한국어 문체', koreanTargetOnly: true },
    { errorMessage: '기본 지침을 찾을 수 없어요.' },
  )
  expect(screen.getByText('한국어 글에만 적용돼요')).toBeInTheDocument()
  expect(screen.getByText('기본 지침을 찾을 수 없어요.')).toBeInTheDocument()
})

// GEN-73: the memories default says it reaches only a post that uses memories, as the
// Korean-only one says its own condition.
it('says a memories-only default applies only to a post that uses memories', () => {
  renderRow({
    ...NAMING,
    key: 'memory_impressions',
    name: '기억을 통한 감상 추가',
    memoriesOnly: true,
  })
  expect(screen.getByText('기억 사용을 켠 글에만 적용돼요')).toBeInTheDocument()
  expect(screen.queryByText('한국어 글에만 적용돼요')).not.toBeInTheDocument()
})
