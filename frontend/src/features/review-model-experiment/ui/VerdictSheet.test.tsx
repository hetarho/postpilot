import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import type { CandidateBadges, ModelExperiment } from '@/entities/model-experiment'
import { BADGE_NOTE_MAX_LENGTH } from '@/entities/model-experiment'
import { VerdictSheet } from './VerdictSheet'

const candidate = (id: string, side: 'left' | 'right') => ({
  id,
  displaySide: side,
  status: 'succeeded' as const,
  badges: [],
  otherNote: '',
  failure: undefined,
  modelLabel: '',
})

const pair: ModelExperiment = {
  id: 'experiment-1',
  stage: 'write',
  origin: 'lab',
  status: 'review',
  postSlug: 'post',
  voiceId: '',
  templateName: '',
  jobId: 'job',
  winnerCandidateId: '',
  outcome: '',
  applyFailure: undefined,
  appliedAt: '',
  adoptionRequested: false,
  adoptionFailure: undefined,
  adoptedAt: '',
  createdAt: '',
  finishedAt: '',
  decidedAt: '',
  revealed: false,
  targetLanguage: 'ko',
  candidates: [candidate('left', 'left'), candidate('right', 'right')],
}

function renderSheet(experiment = pair) {
  const onConfirm = vi.fn<(badges: CandidateBadges[]) => void>()
  const onClose = vi.fn()
  render(
    <VerdictSheet
      experiment={experiment}
      chosenCandidateId="left"
      title="이 결과로 선택"
      confirmLabel="이 결과로 선택"
      open
      pending={false}
      onConfirm={onConfirm}
      onClose={onClose}
    />,
  )
  return { onConfirm, onClose }
}

// Both candidates are offered the catalog: why a result lost is worth as much as why the
// other won, and one verdict carries both answers.
it('collects badges for the chosen and the unchosen candidate at once', async () => {
  const user = userEvent.setup()
  const { onConfirm } = renderSheet()
  const chosen = screen.getByRole('heading', { name: '선택한 결과 A' }).parentElement!
  const unchosen = screen.getByRole('heading', { name: '선택하지 않은 결과 B' }).parentElement!

  await user.click(within(chosen).getByRole('button', { name: '속도가 빨라요' }))
  await user.click(within(chosen).getByRole('button', { name: '문체가 잘 맞아요' }))
  await user.click(within(unchosen).getByRole('button', { name: 'AI 같아요' }))
  expect(within(chosen).getByRole('button', { name: '속도가 빨라요' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await user.click(screen.getByRole('button', { name: '이 결과로 선택' }))
  expect(onConfirm).toHaveBeenCalledWith([
    { candidateId: 'left', badges: ['fast', 'in_voice'], otherNote: '' },
    { candidateId: 'right', badges: ['ai_like'], otherNote: '' },
  ])
})

// Zero badges confirm as readily as ten: a badge is evidence offered, never a toll.
it('confirms with nothing chosen', async () => {
  const user = userEvent.setup()
  const { onConfirm } = renderSheet()
  const confirm = screen.getByRole('button', { name: '이 결과로 선택' })
  expect(confirm).toBeEnabled()
  await user.click(confirm)
  expect(onConfirm).toHaveBeenCalledWith([])
})

// Cancel changes nothing: the verdict is the sheet's confirm, not the button that opened it.
it('commits nothing when it is cancelled', async () => {
  const user = userEvent.setup()
  const { onConfirm, onClose } = renderSheet()
  await user.click(screen.getAllByRole('button', { name: '속도가 빨라요' })[0])
  await user.click(screen.getByRole('button', { name: '취소' }))
  expect(onClose).toHaveBeenCalled()
  expect(onConfirm).not.toHaveBeenCalled()
})

// 기타 opens a bounded field, and the note rides along with the badge that explains it.
it('carries a bounded note only while 기타 is pressed', async () => {
  const user = userEvent.setup()
  const { onConfirm } = renderSheet()
  const chosen = screen.getByRole('heading', { name: '선택한 결과 A' }).parentElement!
  expect(within(chosen).queryByLabelText('기타 이유')).not.toBeInTheDocument()
  await user.click(within(chosen).getByRole('button', { name: '기타' }))
  const note = within(chosen).getByLabelText('기타 이유')
  await user.type(note, '제목이 비슷해요')
  await user.click(screen.getByRole('button', { name: '이 결과로 선택' }))
  expect(onConfirm).toHaveBeenCalledWith([
    { candidateId: 'left', badges: ['other'], otherNote: '제목이 비슷해요' },
  ])
})

it('refuses to confirm a note past its bound', async () => {
  const user = userEvent.setup()
  const { onConfirm } = renderSheet()
  await user.click(screen.getAllByRole('button', { name: '기타' })[0])
  const note = screen.getByLabelText('기타 이유')
  await user.clear(note)
  await user.paste('가'.repeat(BADGE_NOTE_MAX_LENGTH + 1))
  expect(screen.getByRole('button', { name: '이 결과로 선택' })).toBeDisabled()
  expect(onConfirm).not.toHaveBeenCalled()
})

// An observe comparison writes no prose, so neither judgement about voice can be made of it.
it('hides the voice pair for a comparison that wrote no prose', () => {
  renderSheet({ ...pair, stage: 'observe' })
  expect(screen.queryByRole('button', { name: '문체가 잘 맞아요' })).not.toBeInTheDocument()
  expect(screen.queryByRole('button', { name: '문체가 안 맞아요' })).not.toBeInTheDocument()
  expect(screen.getAllByRole('button', { name: '속도가 빨라요' })).toHaveLength(2)
})
