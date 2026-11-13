import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { ModelResultsState } from './ModelResultsState'

it('shows one busy status, then offers retry on failure and renders results on success', async () => {
  const retry = vi.fn()
  const { rerender } = render(
    <ModelResultsState isPending isError={false} onRetry={retry}>
      Results
    </ModelResultsState>,
  )
  expect(screen.getAllByRole('status')).toHaveLength(1)
  expect(screen.getByRole('status').parentElement).toHaveAttribute('aria-busy', 'true')
  rerender(
    <ModelResultsState isPending={false} isError onRetry={retry}>
      Results
    </ModelResultsState>,
  )
  expect(screen.getByRole('alert')).toBeInTheDocument()
  await userEvent.click(screen.getByRole('button'))
  expect(retry).toHaveBeenCalledOnce()
  rerender(
    <ModelResultsState isPending={false} isError={false} onRetry={retry}>
      Results
    </ModelResultsState>,
  )
  expect(screen.getByText('Results')).toBeInTheDocument()
})
