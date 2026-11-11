import { render, screen } from '@testing-library/react'
import { expect, it } from 'vitest'
import { StepScreen } from './StepScreen'

function Flow({ step }: { step: 'kind' | 'photos' }) {
  return (
    <>
      <button type="button">다음</button>
      <StepScreen hidden={step !== 'kind'} title="어떤 글을 쓰시나요?">
        <p>kind</p>
      </StepScreen>
      <StepScreen hidden={step !== 'photos'} title="사진을 골라 주세요">
        <p>photos</p>
      </StepScreen>
    </>
  )
}

// THEME-55: moving to a screen moves focus to its question, so a screen reader hears where the
// owner now is; the first paint takes no focus.
it('focuses the new question on a move, never on the first paint', () => {
  const view = render(<Flow step="kind" />)
  const next = screen.getByRole('button', { name: '다음' })
  next.focus()
  expect(next).toHaveFocus()
  view.rerender(<Flow step="photos" />)
  expect(screen.getByRole('heading', { name: '사진을 골라 주세요' })).toHaveFocus()
  view.rerender(<Flow step="kind" />)
  expect(screen.getByRole('heading', { name: '어떤 글을 쓰시나요?' })).toHaveFocus()
})
