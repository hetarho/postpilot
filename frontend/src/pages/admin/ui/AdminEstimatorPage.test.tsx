import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { chooseOption } from '@/test/listbox'

const MASTER = { id: 'root', plan: ProtoPlan.MASTER }

/** Two models, each registered to exactly one of the stages a combo needs. */
const COMBO_CATALOG = [
  {
    modelId: 'vendor/eyes',
    label: 'Eyes',
    vision: true,
    curated: true,
    purposes: ['photo-analysis'],
  },
  { modelId: 'vendor/pen', label: 'Pen', curated: true, purposes: ['writing'] },
]

describe('the estimator combos', () => {
  // QUOTA-39: the operator names the models, the product names the tier. Both halves go in
  // one call, so choosing the first alone must not fire a refusal.
  it('assigns a pair once both halves are chosen', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/estimator', {
      user: MASTER,
      calls,
      modelCatalog: { entries: COMBO_CATALOG },
    })

    const section = within(await screen.findByRole('region', { name: '편수 기준 조합' }))
    const rows = section.getAllByRole('group')
    expect(rows).toHaveLength(4)
    expect(within(rows[0]).getByRole('heading', { name: '품질' })).toBeInTheDocument()
    expect(
      within(rows[0]).getByText(
        '두 모델을 모두 고르기 전에는 플랜 화면에서 이 등급의 편수를 보여주지 않습니다.',
      ),
    ).toBeInTheDocument()

    await chooseOption(user, within(rows[0]).getByRole('combobox', { name: /사진 분석/ }), 'Eyes')
    // One half is a draft, not a request: the server takes a complete assignment.
    expect(calls.filter((call) => call.startsWith('SetEstimatorCombo'))).toHaveLength(0)

    await chooseOption(user, within(rows[0]).getByRole('combobox', { name: /글 작성/ }), 'Pen')
    await waitFor(() => expect(calls).toContain('SetEstimatorCombo:quality:vendor/eyes/vendor/pen'))
  })

  it('keeps an assignment the server refused out of the section and says so', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/estimator', {
      user: MASTER,
      modelCatalog: { entries: COMBO_CATALOG, comboWriteFails: true },
    })

    const section = within(await screen.findByRole('region', { name: '편수 기준 조합' }))
    const row = within(section.getAllByRole('group')[1])
    await chooseOption(user, row.getByRole('combobox', { name: /사진 분석/ }), 'Eyes')
    await chooseOption(user, row.getByRole('combobox', { name: /글 작성/ }), 'Pen')

    expect(await section.findByRole('alert')).toBeInTheDocument()
  })

  // A tier already assigned shows its two models, which is how the operator sees what the
  // comparison screen is pricing with.
  it('shows what each tier is already priced with', async () => {
    renderAppAt('/admin/estimator', {
      user: MASTER,
      modelCatalog: {
        entries: COMBO_CATALOG,
        estimatorCombos: [
          { combo: 'value', observeModelId: 'vendor/eyes', writeModelId: 'vendor/pen' },
        ],
      },
    })

    const section = within(await screen.findByRole('region', { name: '편수 기준 조합' }))
    // The assignment arrives with the catalog read, so the row re-seeds once it lands.
    await section.findByRole('combobox', { name: /사진 분석 Eyes/ })
    const row = within(section.getAllByRole('group')[2])
    expect(row.getByRole('heading', { name: '가성비' })).toBeInTheDocument()
    expect(row.getByRole('combobox', { name: /사진 분석 Eyes/ })).toBeInTheDocument()
    expect(row.getByRole('combobox', { name: /글 작성 Pen/ })).toBeInTheDocument()
    expect(
      row.queryByText(
        '두 모델을 모두 고르기 전에는 플랜 화면에서 이 등급의 편수를 보여주지 않습니다.',
      ),
    ).not.toBeInTheDocument()
  })
})
