import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
import { RankedReviewSheet } from './RankedReviewSheet'

it('replaces an already-open ranking sheet with a read-only destination and never confirms ranks', async () => {
  const onConfirm = vi.fn()
  const onClose = vi.fn()
  render(
    <RankedReviewSheet
      experiment={{ candidates: [] } as unknown as ModelExperiment}
      open
      pending={false}
      onConfirm={onConfirm}
      onClose={onClose}
    />,
  )
  expect(screen.getByRole('dialog', { name: '이전 유료 비교 기록' })).toBeInTheDocument()
  expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '순위 저장' })).not.toBeInTheDocument()
  expect(screen.getByRole('link', { name: '글쓰기 테스트 시작하기' })).toHaveAttribute(
    'href',
    '/tests',
  )
  await userEvent.setup().click(screen.getAllByRole('button', { name: '닫기' }).at(-1)!)
  expect(onClose).toHaveBeenCalledOnce()
  expect(onConfirm).not.toHaveBeenCalled()
})
