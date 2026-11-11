import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StepBar, StepBarActions, StepBarBack, StepBarCta, StepBarReason } from './StepBar'

describe('StepBar', () => {
  // Owner decision 2026-11-11: every stepped screen ends on this one bar — the step dock, a round
  // ← and the screen's one big action filling the row, with what explains the action above it.
  it('is the step dock holding a round way back and the big action that fills the row', async () => {
    const user = userEvent.setup()
    const back = vi.fn()
    render(
      <StepBar ariaLabel="다음 단계">
        <StepBarReason id="reason">하나를 골라 주세요.</StepBarReason>
        <StepBarActions back={<StepBarBack label="이전" onClick={back} />}>
          <StepBarCta disabled aria-describedby="reason">
            다음
          </StepBarCta>
        </StepBarActions>
      </StepBar>,
    )

    const bar = screen.getByLabelText('다음 단계')
    expect(bar).toHaveAttribute('data-action-bar', 'step')
    expect(bar).toHaveClass('sticky', 'bottom-0', 'mt-auto')
    const previous = screen.getByRole('button', { name: '이전' })
    expect(previous).toHaveClass('ui-button-icon')
    expect(previous).toHaveTextContent('')
    const next = screen.getByRole('button', { name: '다음' })
    expect(next).toHaveClass('ui-button-cta', 'min-h-14', 'flex-1')
    expect(next).toBeDisabled()
    expect(next).toHaveAccessibleDescription('하나를 골라 주세요.')
    // The reason stands above the row holding the action it explains (THEME-31).
    expect(
      screen.getByText('하나를 골라 주세요.').compareDocumentPosition(previous) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    expect(previous.parentElement).toBe(next.parentElement)

    await user.click(previous)
    expect(back).toHaveBeenCalledOnce()
  })
})
