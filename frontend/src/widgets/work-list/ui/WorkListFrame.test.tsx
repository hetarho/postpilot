import { render, screen, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { WorkListLoading } from './WorkListFrame'

vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key: string) => key }) }))

it('keeps one live region and reserves hidden work geometry only while pending', async () => {
  const { container, rerender } = render(<WorkListLoading pending />)
  const status = screen.getByRole('status')
  await waitFor(() => expect(status).toHaveTextContent('state.loading'))
  expect(status.closest('[aria-busy="true"]')).toBeNull()
  const skeleton = container.querySelector('[data-work-skeleton]')
  expect(skeleton).toHaveAttribute('aria-hidden', 'true')
  expect(skeleton?.querySelectorAll('.loading-skeleton')).toHaveLength(7)
  rerender(<WorkListLoading pending={false} />)
  expect(screen.getByRole('status')).toBe(status)
  expect(status).toBeEmptyDOMElement()
  expect(status.parentElement).toHaveClass('sr-only')
  expect(container.querySelector('[data-work-skeleton]')).toBeNull()
})
