import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
import { VerdictSheet } from './VerdictSheet'

it('cannot confirm a legacy winner or create a new legacy verdict', async () => {
  const onConfirm = vi.fn()
  const onClose = vi.fn()
  render(
    <VerdictSheet
      experiment={{ candidates: [] } as unknown as ModelExperiment}
      chosenCandidateId="a"
      title="이 결과로 선택"
      confirmLabel="이 결과로 선택"
      open
      pending={false}
      onConfirm={onConfirm}
      onClose={onClose}
    />,
  )
  expect(screen.getByRole('dialog', { name: '이전 유료 비교 기록' })).toHaveTextContent(
    '열람과 복사만',
  )
  expect(screen.queryByRole('button', { name: '이 결과로 선택' })).not.toBeInTheDocument()
  await userEvent.setup().click(screen.getAllByRole('button', { name: '닫기' }).at(-1)!)
  expect(onClose).toHaveBeenCalledOnce()
  expect(onConfirm).not.toHaveBeenCalled()
})
