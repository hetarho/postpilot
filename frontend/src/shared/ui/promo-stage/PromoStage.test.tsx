import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PromoStage } from './PromoStage'

describe('PromoStage', () => {
  // The aurora is weather behind the options: three blobs of the promotional hues, each on its
  // own period, moving on `transform` alone, clipped to a recessed panel that sits BEHIND the
  // content the stage holds (THEME-37).
  it('drifts three blurred blobs on a recessed panel behind its content', () => {
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
    const blobs = Array.from(aurora.children) as HTMLElement[]
    expect(blobs).toHaveLength(3)
    expect(blobs.map((blob) => blob.className)).toEqual([
      expect.stringContaining('bg-promo-aurora-1 animate-aurora-1'),
      expect.stringContaining('bg-promo-aurora-2 animate-aurora-2'),
      expect.stringContaining('bg-promo-aurora-3 animate-aurora-3'),
    ])
    for (const blob of blobs) expect(blob).toHaveClass('blur-3xl', 'rounded-full')
    // The content is a sibling of the aurora, not inside it, so the clip never touches it.
    expect(screen.getByText('Pro').parentElement).toBe(stage)
  })
})
