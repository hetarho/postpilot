import { render, screen } from '@testing-library/react'
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
})
