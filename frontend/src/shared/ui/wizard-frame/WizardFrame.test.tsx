import { render, screen, within } from '@testing-library/react'
import { WizardFrame } from './WizardFrame'

describe('WizardFrame', () => {
  // The template wizard's first questions do not know the count yet, and the job bar's endless
  // sweep under 어떤 글인지 고르는 중 read as a load that never finished.
  it('says an unknown count in words over a still track, with no running bar', () => {
    const { container } = render(
      <WizardFrame
        screen="target"
        title="어떤 체험을 글로 남기나요?"
        progress={{ text: '어떤 글인지 고르는 중', label: '템플릿 만들기 진행' }}
      />,
    )

    expect(screen.getByText('어떤 글인지 고르는 중')).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    expect(container.querySelector('.animate-progress-sweep')).toBeNull()
  })

  it('reports a known count as a progressbar', () => {
    render(
      <WizardFrame
        screen="name"
        title="이름을 지어 주세요"
        progress={{ text: '3 / 5', label: '템플릿 만들기 진행', current: 3, total: 5 }}
      />,
    )

    const bar = screen.getByRole('progressbar', { name: '템플릿 만들기 진행' })
    expect(bar).toHaveAttribute('aria-valuenow', '3')
    expect(bar).toHaveAttribute('aria-valuemax', '5')
  })

  // Owner decision 2026-11-11: 이전 and 다음 used to sit at the foot of the page, under the body,
  // where they scrolled away; they are the step bar every stepped screen shares, with a held
  // 다음's reason above it and the phone's preview between the two.
  it('puts the way back, the big action and its reason in the step bar', () => {
    render(
      <WizardFrame
        screen="target:1"
        title="어떤 식당인가요?"
        progress={{ text: '어떤 글인지 고르는 중', label: '템플릿 만들기 진행' }}
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
    expect(within(bar).queryByText('식사')).toBeNull()
  })
})
