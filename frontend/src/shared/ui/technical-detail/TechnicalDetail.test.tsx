import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { TechnicalDetail } from './TechnicalDetail'

describe('TechnicalDetail', () => {
  it('renders external diagnostics as escaped inert text', () => {
    const detail = '<img src=x onerror=alert(1)>'
    const { container } = render(<TechnicalDetail label="Technical detail" detail={detail} />)

    expect(screen.getByText('Technical detail')).toBeInTheDocument()
    expect(screen.getByText(detail)).toBeInTheDocument()
    expect(container.querySelector('img')).toBeNull()
  })

  it('renders nothing when no detail is available', () => {
    const { container } = render(<TechnicalDetail label="Technical detail" />)
    expect(container).toBeEmptyDOMElement()
  })
})
