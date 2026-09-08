import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PromoStage } from './PromoStage'
import { startAurora } from './aurora'

describe('PromoStage', () => {
  // The aurora is weather behind the options: a shader canvas over three CSS blobs of the same
  // promotional hues, clipped to a recessed panel that sits BEHIND the content the stage holds,
  // under a film of grain (THEME-37).
  it('layers the shader canvas, the blob fallback and the grain on a recessed panel behind its content', () => {
    const { container } = render(
      <PromoStage>
        <p>Pro</p>
      </PromoStage>,
    )

    const stage = container.firstElementChild as HTMLElement
    expect(stage).toHaveClass('relative', 'isolate')
    const aurora = stage.querySelector('[data-promo-aurora]') as HTMLElement
    expect(aurora).toHaveAttribute('aria-hidden', 'true')
    expect(aurora).toHaveClass(
      'bg-surface-recessed',
      '-z-10',
      'overflow-hidden',
      'pointer-events-none',
    )

    const blobs = Array.from(aurora.querySelectorAll('div')) as HTMLElement[]
    expect(blobs).toHaveLength(3)
    expect(blobs.map((blob) => blob.className)).toEqual([
      expect.stringContaining('bg-promo-aurora-1 animate-aurora-1'),
      expect.stringContaining('bg-promo-aurora-2 animate-aurora-2'),
      expect.stringContaining('bg-promo-aurora-3 animate-aurora-3'),
    ])
    for (const blob of blobs) expect(blob).toHaveClass('blur-3xl', 'rounded-full')

    // The grain: a static turbulence texture blended over the whole panel.
    const grain = aurora.querySelector('[data-promo-grain]') as SVGElement
    expect(grain).toHaveClass('mix-blend-overlay')
    expect(grain.querySelector('feTurbulence')).not.toBeNull()

    // The content is a sibling of the aurora, not inside it, so the clip never touches it.
    expect(screen.getByText('Pro').parentElement).toBe(stage)
  })

  // Without WebGL the canvas never announces itself and the blobs stay the picture: the stage is
  // never blank, and nothing here has a browser to draw with.
  it('keeps the CSS fallback showing where the shader cannot start', () => {
    const { container } = render(
      <PromoStage>
        <p>Pro</p>
      </PromoStage>,
    )
    const canvas = container.querySelector('[data-promo-shader]') as HTMLCanvasElement
    expect(canvas).toHaveAttribute('data-live', 'false')
    expect(canvas).toHaveClass('opacity-0')
    expect(startAurora(canvas)).toBeNull()
  })
})
