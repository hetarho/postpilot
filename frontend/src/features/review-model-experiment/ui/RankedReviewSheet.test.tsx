import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import type { ModelExperiment } from '@/entities/model-experiment'
import { chooseOption } from '@/test/listbox'
import { RankedReviewSheet } from './RankedReviewSheet'

const experiment = {
  stage: 'write',
  candidates: (['left', 'right', 'c', 'd', 'e'] as const).map((side) => ({
    id: side,
    displaySide: side,
    status: 'succeeded',
    badges: [],
    otherNote: '',
  })),
} as unknown as ModelExperiment

it('submits every successful candidate with dense ranks and ties', async () => {
  const user = userEvent.setup()
  const onConfirm = vi.fn().mockResolvedValue(undefined)
  const onClose = vi.fn()
  render(
    <RankedReviewSheet
      experiment={experiment}
      open
      pending={false}
      onConfirm={onConfirm}
      onClose={onClose}
    />,
  )
  const save = screen.getByRole('button', { name: '순위 저장' })
  expect(save).toBeDisabled()
  expect(screen.getAllByRole('button', { name: '속도가 빨라요' })[0]).toBeDisabled()
  for (const [label, rank] of [
    ['A', 1],
    ['B', 1],
    ['C', 2],
    ['D', 3],
    ['E', 3],
  ] as const) {
    await chooseOption(
      user,
      screen.getByRole('combobox', { name: new RegExp(`후보 ${label} 순위`) }),
      `${rank}위`,
    )
  }
  expect(save).toBeEnabled()
  await user.click(save)
  await waitFor(() => expect(onConfirm).toHaveBeenCalledTimes(1))
  expect(onConfirm.mock.calls[0][0]).toEqual(
    ['left', 'right', 'c', 'd', 'e'].map((candidateId, index) => ({
      candidateId,
      rank: [1, 1, 2, 3, 3][index],
      badges: [],
      otherNote: '',
    })),
  )
  await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1))
})

it('keeps ranks selected when the server refuses completion', async () => {
  const user = userEvent.setup()
  const onClose = vi.fn()
  const onConfirm = vi.fn().mockRejectedValue(new Error('rejected'))
  render(
    <RankedReviewSheet
      experiment={{ ...experiment, candidates: experiment.candidates.slice(0, 2) }}
      open
      pending={false}
      onConfirm={onConfirm}
      onClose={onClose}
    />,
  )
  await chooseOption(user, screen.getByRole('combobox', { name: /후보 A 순위/ }), '1위')
  await chooseOption(user, screen.getByRole('combobox', { name: /후보 B 순위/ }), '2위')
  await user.click(screen.getByRole('button', { name: '순위 저장' }))
  await waitFor(() => expect(onConfirm).toHaveBeenCalledOnce())
  expect(onClose).not.toHaveBeenCalled()
  expect(screen.getByRole('combobox', { name: /후보 A 순위/ })).toHaveTextContent('1위')
})
