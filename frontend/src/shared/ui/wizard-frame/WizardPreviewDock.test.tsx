import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { WizardPreviewDock } from './WizardPreviewDock'

// TMPL-75 / CLIP-221: below `lg` a wizard's preview opens from a floating button.
describe('the phone preview button', () => {
  it('opens the preview in a sheet and glows only after a change', () => {
    const { rerender } = render(
      <WizardPreviewDock changes={0} label="미리보기" closeLabel="닫기">
        <p>미리 본 글</p>
      </WizardPreviewDock>,
    )
    expect(screen.queryByTestId('preview-glow')).not.toBeInTheDocument()
    expect(screen.queryByText('미리 본 글')).not.toBeInTheDocument()
    rerender(
      <WizardPreviewDock changes={1} label="미리보기" closeLabel="닫기">
        <p>미리 본 글</p>
      </WizardPreviewDock>,
    )
    expect(screen.getByTestId('preview-glow')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '미리보기' }))
    expect(screen.getByRole('dialog', { name: '미리보기' })).toHaveTextContent('미리 본 글')
    fireEvent.click(screen.getByRole('button', { name: '닫기' }))
    expect(screen.queryByRole('dialog', { name: '미리보기' })).not.toBeInTheDocument()
  })
})
