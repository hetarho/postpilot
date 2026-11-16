import { render, screen, within } from '@testing-library/react'
import { WizardFrame } from './WizardFrame'

const STEPS = [
  { value: 'target:0', label: '종류' },
  { value: 'target:1', label: '세부 종류' },
  { value: 'sections', label: '구성' },
]

describe('WizardFrame', () => {
  // Every guided flow shows where the owner is the same way: one segment per question, the one on
  // screen named as the current step, with its position over the question.
  it('shows one step per question with the current one marked and its position', () => {
    render(
      <WizardFrame
        screen="target:1"
        title="어떤 식당인가요?"
        progress={{
          steps: STEPS,
          current: 'target:1',
          label: '템플릿 만들기 진행',
          position: '2 / 3',
        }}
      />,
    )

    const progress = screen.getByRole('navigation', { name: '템플릿 만들기 진행' })
    expect(within(progress).getAllByRole('listitem')).toHaveLength(3)
    expect(within(progress).getByText('세부 종류').closest('[aria-current="step"]')).not.toBeNull()
    expect(screen.getByText('2 / 3')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  // Owner decision 2026-11-11: 이전 and 다음 used to sit at the foot of the page, under the body,
  // where they scrolled away; they are the step bar every stepped screen shares, with the phone's
  // preview between the two. A held 다음's reason names it for a screen reader without a line
  // that appears and vanishes above the row.
  it('puts the way back and the big action in the step bar, naming why it is held', () => {
    render(
      <WizardFrame
        screen="target:1"
        title="어떤 식당인가요?"
        progress={{ steps: STEPS, current: 'target:1', label: '템플릿 만들기 진행' }}
        back={{ label: '이전', onClick: () => {} }}
        next={{ label: '다음', onClick: () => {}, disabled: true, reason: '하나를 골라 주세요.' }}
        barAction={<button type="button">미리보기</button>}
        barFooter={<button type="button">지금은 건너뛰기</button>}
      >
        <p>식사</p>
      </WizardFrame>,
    )

    const next = screen.getByRole('button', { name: '다음' })
    const bar = next.closest<HTMLElement>('[data-action-bar="step"]')!
    expect(bar).not.toBeNull()
    expect(bar.closest('section')).toHaveClass('flex', 'flex-1', 'flex-col')
    expect(
      within(bar)
        .getAllByRole('button')
        .map((button) => button.textContent || button.ariaLabel),
    ).toEqual(['이전', '미리보기', '다음', '지금은 건너뛰기'])
    expect(within(bar).getByRole('button', { name: '이전' })).toHaveClass('ui-button-icon')
    expect(next).toHaveClass('ui-button-cta')
    expect(next).toBeDisabled()
    expect(next).toHaveAccessibleDescription('하나를 골라 주세요.')
    expect(within(bar).getByText('하나를 골라 주세요.').closest('.sr-only')).not.toBeNull()
    expect(within(bar).queryByText('식사')).toBeNull()
  })
})
