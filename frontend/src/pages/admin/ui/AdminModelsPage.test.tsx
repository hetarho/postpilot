import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { chooseOption } from '@/test/listbox'

const MASTER = { id: 'root', plan: ProtoPlan.MASTER }

/** A catalog with one model from each of three vendors: two featured (openai, anthropic) and one
 *  that is not, so the ordering rule is observable. */
const CATALOG = [
  { modelId: 'nobody/plain-1', label: 'Plain 1', sourceCreatedAt: 500n },
  { modelId: 'anthropic/claude-x', label: 'Claude X', vision: true, sourceCreatedAt: 300n },
  {
    modelId: 'openai/gpt-old',
    label: 'GPT Old',
    vision: true,
    structuredOutput: true,
    sourceCreatedAt: 100n,
  },
  {
    modelId: 'openai/gpt-new',
    label: 'GPT New',
    structuredOutput: true,
    contextTokens: 200000n,
    inputUsdPerMillion: '1.25',
    outputUsdPerMillion: '4.25',
    sourceCreatedAt: 900n,
  },
]

function modelNames() {
  return screen.getAllByRole('listitem').map((row) => within(row).getByText(/\//).textContent ?? '')
}

describe('the model catalog tab', () => {
  // A2: featured vendors first in the configured order, then the rest alphabetically, newest
  // model first inside a vendor.
  it('orders featured providers first and the newest model first within one', async () => {
    renderAppAt('/admin/models', { user: MASTER, modelCatalog: { entries: CATALOG } })

    await screen.findByRole('heading', { name: '모델 관리' })
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(4))
    expect(modelNames()).toEqual([
      'openai/gpt-new',
      'openai/gpt-old',
      'anthropic/claude-x',
      'nobody/plain-1',
    ])
  })

  // The provider offers several hundred models, and mounting every row made the screen lag on
  // every keystroke. Only the rows near the viewport exist, so the mounted count stays bounded
  // however long the catalog is — while the whole list is still what search and the filters see.
  it('mounts only the rows near the viewport for a large catalog', async () => {
    const user = userEvent.setup()
    const many = Array.from({ length: 400 }, (_, index) => ({
      modelId: `vendor${index % 40}/model-${index}`,
      label: `Model ${index}`,
      sourceCreatedAt: BigInt(400 - index),
    }))
    renderAppAt('/admin/models', { user: MASTER, modelCatalog: { entries: many } })

    await screen.findByRole('heading', { name: '모델 관리' })
    // The count line reports the whole catalog, so the list is complete even though the DOM
    // holds a fraction of it.
    expect(await screen.findByText('400개 중 400개를 보여 주고 있어요.')).toBeInTheDocument()
    await waitFor(() => expect(screen.getAllByRole('listitem').length).toBeGreaterThan(0))
    expect(screen.getAllByRole('listitem').length).toBeLessThan(60)

    // Filtering reaches models that were never mounted.
    await user.type(screen.getByRole('searchbox', { name: '검색' }), 'model-399')
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(1))
    expect(screen.getByText('vendor39/model-399')).toBeInTheDocument()
  })

  // A2: search and the capability filters narrow the same response, with no further round trip.
  it('narrows the list by search and by capability without asking the server again', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/models', { user: MASTER, calls, modelCatalog: { entries: CATALOG } })

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(4))
    const before = calls.filter((call) => call.startsWith('ListCatalog')).length

    await user.type(screen.getByRole('searchbox', { name: '검색' }), 'gpt')
    await waitFor(() => expect(modelNames()).toEqual(['openai/gpt-new', 'openai/gpt-old']))

    await user.clear(screen.getByRole('searchbox', { name: '검색' }))
    await user.click(screen.getByRole('checkbox', { name: '이미지 입력 가능' }))
    await waitFor(() => expect(modelNames()).toEqual(['openai/gpt-old', 'anthropic/claude-x']))

    expect(calls.filter((call) => call.startsWith('ListCatalog')).length).toBe(before)
  })

  // A3: checking a model enables it. Change 19 removed the tier that used to be part of that
  // decision — a model is no longer gated by plan, so enabling it is the whole choice.
  it('enables a model, with no tier to choose', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/models', {
      user: MASTER,
      calls,
      modelCatalog: { entries: [{ modelId: 'openai/gpt-new', label: 'GPT New' }] },
    })

    const row = await screen.findByRole('listitem')
    await user.click(within(row).getByRole('checkbox', { name: '사용' }))
    expect(calls).toContain('EnableModel')

    expect(screen.queryByRole('combobox', { name: /필요한 플랜/ })).not.toBeInTheDocument()
  })

  // A11: the per-model reasoning override round-trips through the same edit, and "stage default"
  // is a real choice rather than an absent one.
  it('sets and clears the reasoning override on an enabled model', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/models', {
      user: MASTER,
      calls,
      modelCatalog: {
        entries: [
          {
            modelId: 'anthropic/claude-x',
            label: 'Claude X',
            curated: true,
            enabled: true,
            minPlan: ProtoPlan.MAX,
          },
        ],
      },
    })

    const reasoning = await screen.findByRole('combobox', { name: /추론 강도 단계 기본값/ })
    await chooseOption(user, reasoning, 'unset')
    expect(calls).toContain('UpdateModel')
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: /추론 강도 unset/ })).toBeInTheDocument(),
    )
  })

  // A6: a curated model the provider no longer offers is badged and counted, and nothing is
  // disabled on its behalf.
  it('flags a withdrawn model for the operator instead of retiring it', async () => {
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          {
            modelId: 'anthropic/claude-gone',
            label: 'Claude Gone',
            curated: true,
            enabled: true,
            listed: false,
          },
        ],
      },
    })

    const row = await screen.findByRole('listitem')
    expect(within(row).getByText('제공 종료')).toBeInTheDocument()
    // Still in use: retiring it is the operator's decision, not the screen's.
    expect(within(row).getByRole('checkbox', { name: '사용' })).toBeChecked()
    expect(
      await screen.findByText(/모델 1개를 제공사가 더 이상 제공하지 않아요/),
    ).toBeInTheDocument()
  })

  // A7: an unreadable provider catalog degrades to curated rows and says so, rather than looking
  // like an empty catalog.
  it('degrades to curated rows when the provider catalog cannot be read', async () => {
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        fetchFails: true,
        entries: [{ modelId: 'openai/gpt-new', label: 'GPT New', curated: true, enabled: true }],
      },
    })

    expect(
      await screen.findByText(/제공사의 모델 목록을 읽지 못해서, 이미 등록해 둔 모델만/),
    ).toBeInTheDocument()
    expect(await screen.findByRole('listitem')).toBeInTheDocument()
  })

  // The refresh is the one action that bypasses the server's cache, so it must ask with the flag.
  it('asks the provider again on refresh', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/models', { user: MASTER, calls, modelCatalog: { entries: CATALOG } })

    await user.click(await screen.findByRole('button', { name: '목록 새로고침' }))
    await waitFor(() => expect(calls).toContain('ListCatalog:refresh'))
  })

  // A1: the tab is master-only, redirected rather than refused — the account has a session.
  it('sends a non-operator back to the app', async () => {
    renderAppAt('/admin/models', {
      user: { id: 'alice', plan: ProtoPlan.MAX },
      plans: { plan: ProtoPlan.MAX },
    })

    expect(await screen.findByRole('heading', { name: '내 글' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '모델 관리' })).not.toBeInTheDocument()
  })
})
