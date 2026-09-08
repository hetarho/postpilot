import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { PromoFrame } from './PromoFrame'

describe('PromoFrame', () => {
  // The effect is a STROKE: the content keeps its own opaque surface, so the gradient shows
  // only in the frame's padding and every figure inside keeps the pair it was audited
  // against (THEME-7).
  it('paints a still gradient stroke behind opaque content', () => {
    const { container } = render(
      <PromoFrame>
        <p>가장 합리적</p>
      </PromoFrame>,
    )

    const frame = container.firstElementChild as HTMLElement
    expect(frame).toHaveClass('rounded-lg')
    // Nothing moves on an unmarked frame: no halo, and the stroke is a registered gradient
    // painted once, not a layer turning behind the content (owner decision 2026-09-09).
    expect(frame.querySelector('[data-promo-glow]')).toBeNull()
    const stroke = frame.querySelector('[data-promo-stroke]') as HTMLElement
    expect(stroke).toHaveClass('bg-promo-stroke', 'p-px')
    expect(stroke.className).not.toMatch(/animate-/)

    const content = screen.getByText('가장 합리적').parentElement as HTMLElement
    expect(content).toHaveClass('bg-surface-raised', 'isolate')
    expect(content.parentElement).toBe(stroke)

    // The spotlight: a tint of the accent under the pointer, shown only where a pointer can
    // hover, painted between the surface and the text so nothing it passes under loses contrast.
    expect(frame).toHaveClass('group')
    const spot = content.querySelector('[data-promo-spot]') as HTMLElement
    expect(spot).toHaveAttribute('aria-hidden', 'true')
    // `bg-promo-spotlight`, the gradient — not `bg-promo-spot`, which Tailwind resolves to the
    // COLOUR role of that name and which filled every card solid on hover.
    expect(spot).toHaveClass('bg-promo-spotlight', 'opacity-0', 'group-hover:opacity-100', '-z-10')
    expect(spot).not.toHaveClass('bg-promo-spot')
  })

  it('writes the pointer position into the frame and tilts toward it as a mouse moves', () => {
    const { container } = render(
      <PromoFrame>
        <p>Pro</p>
      </PromoFrame>,
    )
    const frame = container.firstElementChild as HTMLElement
    frame.getBoundingClientRect = () => ({ left: 100, top: 50, width: 200, height: 100 }) as DOMRect
    // Upper-right quadrant: the card tips its top away and its right side away, both toward
    // the pointer, at a fraction of the 7° maximum.
    fireEvent.pointerMove(frame, { clientX: 250, clientY: 60, pointerType: 'mouse' })
    expect(frame.style.getPropertyValue('--spot-x')).toBe('150px')
    expect(frame.style.getPropertyValue('--spot-y')).toBe('10px')
    // x = 150 of 200 → +0.25 of the width → a quarter of the maximum; y = 10 of 100 → -0.4.
    expect(frame.style.transform).toBe('perspective(900px) rotateX(2.80deg) rotateY(1.75deg)')

    // The pointer leaving settles the card flat again.
    fireEvent.pointerLeave(frame)
    expect(frame.style.transform).toBe('')
  })

  // A finger covers what it presses and has no hover to leave, so a touch moves the spot's
  // anchor and nothing else.
  it('does not tilt for a touch pointer', () => {
    const { container } = render(
      <PromoFrame>
        <p>Pro</p>
      </PromoFrame>,
    )
    const frame = container.firstElementChild as HTMLElement
    frame.getBoundingClientRect = () => ({ left: 0, top: 0, width: 200, height: 100 }) as DOMRect
    fireEvent.pointerMove(frame, { clientX: 180, clientY: 20, pointerType: 'touch' })
    expect(frame.style.getPropertyValue('--spot-x')).toBe('180px')
    expect(frame.style.transform).toBe('')
  })

  // One option per view is carried further, and never by the glow alone: the caller puts a
  // text label inside.
  it('widens the stroke, casts a shadow and glows for the marked option', () => {
    const { container } = render(
      <PromoFrame marked>
        <p>Pro</p>
      </PromoFrame>,
    )

    const frame = container.firstElementChild as HTMLElement
    expect(frame).toHaveClass('shadow-md')
    const stroke = frame.querySelector('[data-promo-stroke]') as HTMLElement
    expect(stroke).toHaveClass('p-0.5')
    expect(stroke).not.toHaveClass('p-px')

    // The halo: the same registered gradient, blurred, breathing on opacity alone (THEME-28,
    // THEME-37), hidden from assistive tech because the label beside it carries the meaning.
    const glow = frame.querySelector('[data-promo-glow]') as HTMLElement
    expect(glow).toHaveAttribute('aria-hidden', 'true')
    expect(glow).toHaveClass('bg-promo-stroke', 'animate-promo-glow', 'blur-lg', '-z-10')
  })
})
