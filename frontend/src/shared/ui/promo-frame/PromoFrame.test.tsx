import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PromoFrame } from './PromoFrame'

describe('PromoFrame', () => {
  // The effect is a STROKE: the content keeps its own opaque surface, so the gradient shows
  // only in the frame's padding and every figure inside keeps the pair it was audited
  // against (THEME-7).
  it('paints a rotating layer behind opaque content', () => {
    const { container } = render(
      <PromoFrame>
        <p>가장 합리적</p>
      </PromoFrame>,
    )

    const frame = container.firstElementChild as HTMLElement
    expect(frame).toHaveClass('rounded-lg', 'p-px')

    const stroke = frame.querySelector('[data-promo-stroke]') as HTMLElement
    // Hidden from assistive tech: it carries no meaning of its own.
    expect(stroke).toHaveAttribute('aria-hidden', 'true')
    // Transform-only motion on a registered gradient (THEME-28, THEME-11). Reduced motion is
    // honoured globally in index.css, which freezes this at its first frame and leaves the
    // stroke visible rather than removing it.
    expect(stroke).toHaveClass('bg-promo-stroke', 'animate-promo-spin', 'w-promo-layer')

    const content = screen.getByText('가장 합리적').parentElement as HTMLElement
    expect(content).toHaveClass('bg-surface-raised')
  })

  // One option per view is carried further, and never by the glow alone: the caller puts a
  // text label inside.
  it('widens the stroke and casts a shadow for the marked option', () => {
    const { container } = render(
      <PromoFrame marked>
        <p>Pro</p>
      </PromoFrame>,
    )

    const frame = container.firstElementChild as HTMLElement
    expect(frame).toHaveClass('p-0.5', 'shadow-md')
    expect(frame).not.toHaveClass('p-px')
    expect(frame.querySelector('[data-promo-stroke]')).toBeInTheDocument()
  })
})
