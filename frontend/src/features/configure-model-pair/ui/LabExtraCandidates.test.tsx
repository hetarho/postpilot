import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import { Stage } from '@/shared/api'
import { createFakeProviderTransport } from '@/test/providers'
import { chooseOption } from '@/test/listbox'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useModelSetup } from '@/entities/model-catalog'
import { LabExtraCandidates } from './LabExtraCandidates'
import { labCandidateRefs } from '../model/lab-candidates'

const ref = (modelId: string) => ({ providerId: 'openrouter', modelId })
const pair = { stage: Stage.WRITE, candidateA: ref('a'), candidateB: ref('b') }

function Form() {
  const { pairs } = useModelSetup()
  return (
    <LabExtraCandidates
      stage="write"
      pair={pairs.find((item) => item.stage === 'write')}
      onShownChange={() => {}}
    />
  )
}

it('saves C/D/E in order, caps five total candidates and compacts a removed row', async () => {
  const saved: string[][] = []
  const user = userEvent.setup()
  render(<Form />, {
    wrapper: withProviders(
      createFakeProviderTransport({
        comparisonPairs: [pair],
        models: ['a', 'b', 'c', 'd', 'e'].map((id) => ({ ...ref(id), label: id.toUpperCase() })),
        onSaveLabExtras: (ids) => saved.push(ids),
      }),
      createTestQueryClient(),
    ),
  })
  await screen.findByRole('button', { name: '후보 추가' })
  for (const [label, model] of [
    ['C', 'C'],
    ['D', 'D'],
    ['E', 'E'],
  ] as const) {
    await user.click(screen.getByRole('button', { name: '후보 추가' }))
    await chooseOption(
      user,
      screen.getByRole('combobox', { name: new RegExp(`후보 ${label}`) }),
      model,
    )
    await waitFor(() => expect(saved.at(-1)).toEqual(['c', 'd', 'e'].slice(0, saved.length)))
  }
  expect(screen.queryByRole('button', { name: '후보 추가' })).not.toBeInTheDocument()
  await user.click(screen.getAllByRole('button', { name: '제거' })[0])
  await waitFor(() => expect(saved.at(-1)).toEqual(['d', 'e']))
  expect(screen.getByRole('combobox', { name: /후보 C/ })).toHaveTextContent('D')
})

it('rejects a saved pair collision or unsaved visible C from a start list', () => {
  const base = {
    stage: 'write' as const,
    candidateA: {
      stage: 'write' as const,
      slot: 'candidateA' as const,
      ref: ref('a'),
      missing: false,
    },
    candidateB: {
      stage: 'write' as const,
      slot: 'candidateB' as const,
      ref: ref('b'),
      missing: false,
    },
    extraCandidates: [
      { stage: 'write' as const, slot: 'candidateC' as const, ref: ref('c'), missing: false },
    ],
  }
  expect(labCandidateRefs(base, ['openrouter/c'])).toHaveLength(3)
  expect(labCandidateRefs(base, [''])).toBeUndefined()
  expect(
    labCandidateRefs({ ...base, candidateA: { ...base.candidateA, ref: ref('c') } }, [
      'openrouter/c',
    ]),
  ).toBeUndefined()
  expect(
    labCandidateRefs(
      { ...base, extraCandidates: [{ ...base.extraCandidates[0], missing: true }] },
      ['openrouter/c'],
    ),
  ).toBeUndefined()
})
