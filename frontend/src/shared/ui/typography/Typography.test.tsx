import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { Typography } from './Typography'
import { typographyStyles } from './typographyStyles'

afterEach(cleanup)

describe('Typography', () => {
  it('renders each variant with its THEME-19 recipe and semantic default element', () => {
    render(
      <>
        <Typography variant="display">Page title</Typography>
        <Typography variant="stepTitle">Current task</Typography>
        <Typography variant="title">Section</Typography>
        <Typography variant="fieldTitle">Field</Typography>
        <Typography variant="body">Prose</Typography>
        <Typography variant="label">Label</Typography>
        <Typography variant="meta">Meta</Typography>
        <Typography variant="eyebrow">Eyebrow</Typography>
      </>,
    )
    const display = screen.getByRole('heading', { level: 1, name: 'Page title' })
    expect(display).toHaveClass(
      'text-3xl',
      'sm:text-4xl',
      'font-bold',
      'tracking-tight',
      'leading-tight',
    )
    const step = screen.getByRole('heading', { level: 2, name: 'Current task' })
    expect(step).toHaveClass(
      'text-step-title',
      'sm:text-step-title-wide',
      'font-bold',
      'leading-tight',
    )
    const title = screen.getByRole('heading', { level: 2, name: 'Section' })
    expect(title).toHaveClass('text-title', 'sm:text-2xl', 'font-bold', 'tracking-tight')
    // Peer titles are smaller than the current task and use the same bold emphasis.
    const fieldTitle = screen.getByRole('heading', { level: 3, name: 'Field' })
    expect(fieldTitle).toHaveClass('text-lg', 'font-bold', 'tracking-tight')
    const body = screen.getByText('Prose')
    expect(body.tagName).toBe('P')
    // Guide: 17px reading text.
    expect(body).toHaveClass('text-body', 'leading-relaxed')
    expect(screen.getByText('Label')).toHaveClass(
      'text-label',
      'font-semibold',
      'text-content-secondary',
    )
    expect(screen.getByText('Meta')).toHaveClass('text-meta', 'text-content-tertiary')
    expect(screen.getByText('Eyebrow')).toHaveClass('uppercase', 'tracking-wide')
  })

  it('renders the large title as the page heading, heavier than display', () => {
    render(<Typography variant="largeTitle">네이버 블로그</Typography>)
    expect(screen.getByRole('heading', { level: 1, name: '네이버 블로그' })).toHaveClass(
      'font-extrabold',
      'text-large-title',
    )
  })

  it('separates the visual role from the outline level via `as`', () => {
    render(
      <Typography variant="title" as="h3">
        Deep heading
      </Typography>,
    )
    expect(screen.getByRole('heading', { level: 3, name: 'Deep heading' })).toHaveClass(
      'text-title',
    )
  })

  it('passes through ARIA and merges caller layout classes after the recipe', () => {
    render(
      <Typography variant="meta" role="status" className="text-content-secondary mt-4">
        3 / 8
      </Typography>,
    )
    const element = screen.getByRole('status')
    // twMerge: the caller's colour intent wins over the recipe's, layout classes just append.
    expect(element).toHaveClass('mt-4', 'text-meta', 'text-content-secondary')
    expect(element).not.toHaveClass('text-content-tertiary')
  })

  it('offers the recipes to self-semantic elements through typographyStyles', () => {
    const classes = typographyStyles({ variant: 'label', mono: true, className: 'text-link-fg' })
    expect(classes).toContain('text-label')
    expect(classes).toContain('font-mono')
    expect(classes).toContain('text-link-fg')
    expect(classes).not.toContain('text-content-secondary')
  })
})
