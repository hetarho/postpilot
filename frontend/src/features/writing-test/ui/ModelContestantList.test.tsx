import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { useState } from 'react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import type { CatalogModel, ModelRef, StageName } from '@/entities/model-catalog'
import { writingTestI18n } from '../config/i18n'
import { ModelContestantList } from './ModelContestantList'

beforeEach(() => {
  initializeI18n('ko')
  i18next.addResourceBundle('ko', 'writingTests', writingTestI18n.ko, true, true)
})
afterEach(cleanup)

const ref = (modelId: string): ModelRef => ({ providerId: 'openrouter', modelId })
const model = (
  modelId: string,
  patch: Partial<CatalogModel> & { grade?: string; stage?: StageName } = {},
): CatalogModel => {
  const stage = patch.stage ?? 'write'
  const { grade, stage: _stage, ...rest } = patch
  void _stage
  return {
    ref: ref(modelId),
    label: modelId,
    vision: true,
    videoInput: false,
    structuredOutput: true,
    stages: [stage],
    levels: grade ? { [stage]: grade } : {},
    access: {
      [stage]: { grade: grade ?? 'balanced', entitled: true, requiredPlan: 'pro' },
    } as CatalogModel['access'],
    disabled: false,
    disabledReason: '',
    contextTokens: 0n,
    requiredCredits: 1,
    affordable: true,
    ...rest,
  }
}
const catalog: CatalogModel[] = [
  model('top-model', { grade: 'top', postCredits: { write: { credits: 90, basis: 'recent' } } }),
  model('free-model', { grade: 'free' }),
  model('value-model', {
    grade: 'value',
    postCredits: { write: { credits: 8, basis: 'estimate' } },
  }),
  model('no-figure', { grade: 'premium' }),
  model('locked', { grade: 'premium' }),
  model('photo-only', { stage: 'observe' }),
]

function Harness({
  count = 2,
  initial = [],
  onChange,
  issue = (m: CatalogModel) => (m.ref.modelId === 'locked' ? 'Pro 요금제에서 쓸 수 있어요' : ''),
}: {
  count?: number
  initial?: ModelRef[]
  onChange?: (next: ModelRef[]) => void
  issue?: (model: CatalogModel) => string
}) {
  const [selected, setSelected] = useState(initial)
  return (
    <ModelContestantList
      stage="write"
      count={count}
      models={catalog}
      selected={selected}
      issue={issue}
      onChange={(next) => {
        setSelected(next)
        onChange?.(next)
      }}
    />
  )
}
const boxes = () =>
  screen.getAllByRole('checkbox').map((box) => box.getAttribute('aria-labelledby'))

it('lists the stage rows in grade order with credits per post, 무료 and an unknown-price text', () => {
  render(<Harness />)
  // Free first, then 가성비 up to 최고, ungraded after; a model of another stage is not listed.
  const names = screen
    .getAllByRole('checkbox')
    .map((box) => document.getElementById(box.getAttribute('aria-labelledby')!)!.textContent)
  expect(names).toEqual(['free-model', 'value-model', 'no-figure', 'locked', 'top-model'])
  expect(boxes()).toHaveLength(5)
  expect(screen.getByRole('checkbox', { name: 'free-model' })).toHaveAccessibleDescription('무료')
  expect(screen.getByRole('checkbox', { name: 'value-model' })).toHaveAccessibleDescription(/8/)
  expect(screen.getByRole('checkbox', { name: 'top-model' })).toHaveAccessibleDescription(/90/)
  expect(screen.getByRole('checkbox', { name: 'no-figure' })).toHaveAccessibleDescription(
    '가격 확인 불가',
  )
  expect(screen.getByText('가성비')).toBeVisible()
  expect(screen.getByText('최고')).toBeVisible()
})

it('keeps a locked model as a disabled row stating its reason', () => {
  render(<Harness />)
  const locked = screen.getByRole('checkbox', { name: 'locked' })
  expect(locked).toBeDisabled()
  expect(locked).toHaveAccessibleDescription('Pro 요금제에서 쓸 수 있어요')
})

it('reads checked against the count, refuses a check past it in place and frees a place on uncheck', async () => {
  const onChange = vi.fn()
  render(<Harness onChange={onChange} />)
  expect(screen.getByText('0 / 2개 선택')).toBeVisible()
  await userEvent.click(screen.getByRole('checkbox', { name: 'top-model' }))
  await userEvent.click(screen.getByRole('checkbox', { name: 'free-model' }))
  expect(screen.getByText('2 / 2개 선택')).toBeVisible()
  onChange.mockClear()
  await userEvent.click(screen.getByRole('checkbox', { name: 'value-model' }))
  expect(onChange).not.toHaveBeenCalled()
  expect(screen.getByRole('checkbox', { name: 'value-model' })).not.toBeChecked()
  expect(screen.getByText('2개까지 고를 수 있어요')).toBeVisible()
  await userEvent.click(screen.getByRole('checkbox', { name: 'top-model' }))
  expect(screen.getByText('1 / 2개 선택')).toBeVisible()
  expect(screen.queryByText('2개까지 고를 수 있어요')).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('checkbox', { name: 'value-model' }))
  expect(screen.getByRole('checkbox', { name: 'value-model' })).toBeChecked()
})

it('reports the selection in the order the models were checked', async () => {
  const onChange = vi.fn()
  render(<Harness count={3} onChange={onChange} />)
  for (const name of ['top-model', 'free-model', 'value-model'])
    await userEvent.click(screen.getByRole('checkbox', { name }))
  expect(onChange).toHaveBeenLastCalledWith([
    ref('top-model'),
    ref('free-model'),
    ref('value-model'),
  ])
  await userEvent.click(screen.getByRole('checkbox', { name: 'free-model' }))
  expect(onChange).toHaveBeenLastCalledWith([ref('top-model'), ref('value-model')])
})

it('shows an already selected pair checked', () => {
  render(<Harness initial={[ref('value-model'), ref('top-model')]} />)
  expect(screen.getByRole('checkbox', { name: 'value-model' })).toBeChecked()
  expect(screen.getByRole('checkbox', { name: 'top-model' })).toBeChecked()
  expect(screen.getByText('2 / 2개 선택')).toBeVisible()
})
