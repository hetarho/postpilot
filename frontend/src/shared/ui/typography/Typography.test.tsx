import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { Typography } from './Typography'
import { typographyStyles } from './typographyStyles'

afterEach(cleanup)

describe('Typography', () => {
  it('renders each variant with its §3 recipe and semantic default element', () => {
    render(
      <>
        <Typography variant="display">Page title</Typography>
        <Typography variant="title">Section</Typography>
        <Typography variant="fieldTitle">Field</Typography>
        <Typography variant="body">Prose</Typography>
        <Typography variant="label">Label</Typography>
        <Typography variant="meta">Meta</Typography>
        <Typography variant="eyebrow">Eyebrow</Typography>
      </>,
    )
    const display = screen.getByRole('heading', { level: 1, name: 'Page title' })
    expect(display).toHaveClass('text-2xl', 'font-semibold', 'tracking-tight')
    const title = screen.getByRole('heading', { level: 2, name: 'Section' })
    expect(title).toHaveClass('text-lg', 'font-semibold', 'tracking-tight')
    // Smaller than the step title it stands beside, heavier than a caption (A9).
    const fieldTitle = screen.getByRole('heading', { level: 3, name: 'Field' })
    expect(fieldTitle).toHaveClass('text-base', 'font-bold', 'tracking-tight')
    const body = screen.getByText('Prose')
    expect(body.tagName).toBe('P')
    expect(body).toHaveClass('text-sm', 'leading-relaxed')
    expect(screen.getByText('Label')).toHaveClass('text-sm', 'text-content-secondary')
    expect(screen.getByText('Meta')).toHaveClass('text-xs', 'text-content-tertiary')
    expect(screen.getByText('Eyebrow')).toHaveClass('uppercase', 'tracking-wide')
  })

  it('separates the visual role from the outline level via `as`', () => {
    render(
      <Typography variant="title" as="h3">
        Deep heading
      </Typography>,
    )
    expect(screen.getByRole('heading', { level: 3, name: 'Deep heading' })).toHaveClass('text-lg')
  })

  it('passes through ARIA and merges caller layout classes after the recipe', () => {
    render(
      <Typography variant="meta" role="status" className="text-content-secondary mt-4">
        3 / 8
      </Typography>,
    )
    const element = screen.getByRole('status')
    // twMerge: the caller's colour intent wins over the recipe's, layout classes just append.
    expect(element).toHaveClass('mt-4', 'text-xs', 'text-content-secondary')
    expect(element).not.toHaveClass('text-content-tertiary')
  })

  it('offers the recipes to self-semantic elements through typographyStyles', () => {
    const classes = typographyStyles({ variant: 'label', mono: true, className: 'text-link-fg' })
    expect(classes).toContain('text-sm')
    expect(classes).toContain('font-mono')
    expect(classes).toContain('text-link-fg')
    expect(classes).not.toContain('text-content-secondary')
  })
})
