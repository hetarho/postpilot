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
  // A2 (plan 18) + change 20: featured vendors first in the configured order, then the rest
  // alphabetically, newest model first inside a vendor — on a tab whose gate admits them all.
  it('orders featured providers first and the newest model first within one', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', { user: MASTER, modelCatalog: { entries: CATALOG } })

    await screen.findByRole('heading', { name: '모델 관리' })
    await user.click(await screen.findByRole('tab', { name: '글 작성' }))
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(4))
    expect(modelNames()).toEqual([
      'openai/gpt-new',
      'openai/gpt-old',
      'anthropic/claude-x',
      'nobody/plain-1',
    ])
  })

  // Change 20 A1/A2: the five purpose tabs are one screen; the photo tab is capability-forced
  // to vision models, and a registration made on one tab does not check the box on another.
  it('force-filters each purpose tab and keeps registrations per purpose', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/models', { user: MASTER, calls, modelCatalog: { entries: CATALOG } })

    // The photo-analysis tab is first: only the two vision models are even listed.
    await screen.findByRole('heading', { name: '모델 관리' })
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2))
    expect(modelNames()).toEqual(['openai/gpt-old', 'anthropic/claude-x'])

    const row = screen.getAllByRole('listitem')[0]
    await user.click(within(row).getByRole('checkbox', { name: '이 용도에 사용' }))
    expect(calls).toContain('SetModelPurpose:photo-analysis:register:openai/gpt-old')

    // The same model on the writing tab: listed (no gate), but the box reflects THAT purpose,
    // which was never registered.
    await user.click(screen.getByRole('tab', { name: '글 작성' }))
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(4))
    const writingRow = screen
      .getAllByRole('listitem')
      .find((item) => within(item).queryByText('openai/gpt-old'))
    expect(writingRow).toBeDefined()
    await waitFor(() =>
      expect(
        within(writingRow!).getByRole('checkbox', { name: '이 용도에 사용' }),
      ).not.toBeChecked(),
    )
  })

  // A registered model that later lost the gated capability must stay visible on its tab —
  // that checkbox is the only control that can deregister it.
  it('keeps a registered-but-no-longer-eligible model visible on its tab', async () => {
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          {
            modelId: 'openai/lost-vision',
            label: 'Lost Vision',
            vision: false,
            curated: true,
            purposes: ['photo-analysis'],
          },
        ],
      },
    })

    // The photo-analysis tab is first; the entry fails its gate but is registered to it.
    const row = await screen.findByRole('listitem')
    expect(within(row).getByRole('checkbox', { name: '이 용도에 사용' })).toBeChecked()
  })

  // Change 20 A2: a tab whose gate admits nothing explains itself instead of erroring.
  it('explains an empty purpose tab instead of erroring', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', { user: MASTER, modelCatalog: { entries: CATALOG } })

    await screen.findByRole('heading', { name: '모델 관리' })
    await user.click(await screen.findByRole('tab', { name: '비디오 생성' }))
    expect(
      await screen.findByText('이 용도의 조건을 만족하는 모델이 아직 없어요.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  // The generation tabs are only reachable at all because the catalog is now read with an
  // explicit output_modalities query — the endpoint serves text-output models by default.
  it('lists a video-output model on the video tab and says its price is unpublished', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          ...CATALOG,
          { modelId: 'google/veo-x', label: 'Veo X', videoOutput: true, sourceCreatedAt: 700n },
        ],
      },
    })

    await screen.findByRole('heading', { name: '모델 관리' })
    await user.click(await screen.findByRole('tab', { name: '비디오 생성' }))
    const row = await screen.findByRole('listitem')
    expect(within(row).getByText('google/veo-x')).toBeInTheDocument()
    expect(within(row).getByText('비디오 생성')).toBeInTheDocument()
    // No token price is published for a video model, and $0 would read as free.
    expect(within(row).getByText('토큰 단가 미공개')).toBeInTheDocument()
    expect(within(row).queryByText(/100만 토큰당/)).not.toBeInTheDocument()
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
    await user.click(await screen.findByRole('tab', { name: '글 작성' }))
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

    await screen.findByRole('heading', { name: '모델 관리' })
    await user.click(await screen.findByRole('tab', { name: '글 작성' }))
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(4))
    const before = calls.filter((call) => call.startsWith('ListCatalog')).length

    await user.type(screen.getByRole('searchbox', { name: '검색' }), 'gpt')
    await waitFor(() => expect(modelNames()).toEqual(['openai/gpt-new', 'openai/gpt-old']))

    await user.clear(screen.getByRole('searchbox', { name: '검색' }))
    await user.click(screen.getByRole('checkbox', { name: '이미지 입력 가능' }))
    await waitFor(() => expect(modelNames()).toEqual(['openai/gpt-old', 'anthropic/claude-x']))

    expect(calls.filter((call) => call.startsWith('ListCatalog')).length).toBe(before)
  })

  // A11 (plan 18) + A1/A3 (change 24): the override round-trips through the same edit, "stage
  // default" is a real choice rather than an absent one, and the edit names the PURPOSE it
  // applies to.
  it('sets and clears the reasoning override on a registered model', async () => {
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
            vision: true,
            curated: true,
            purposes: ['photo-analysis'],
          },
        ],
      },
    })

    const reasoning = await screen.findByRole('combobox', { name: /추론 강도 단계 기본값/ })
    await chooseOption(user, reasoning, 'unset')
    expect(calls).toContain('UpdateModel:photo-analysis:unset')
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: /추론 강도 unset/ })).toBeInTheDocument(),
    )
    // Cleared back to the stage policy, which is a real value on the wire.
    await chooseOption(
      user,
      screen.getByRole('combobox', { name: /추론 강도 unset/ }),
      '단계 기본값',
    )
    expect(calls).toContain('UpdateModel:photo-analysis:')
  })

  // A2 — the 2026-09-03 regression, from the operator's side: each purpose tab shows and edits
  // ITS OWN effort. Lowering the effort on 글 작성 must not touch 사진 해석, which is a
  // different task with a different code-owned policy.
  it("shows and edits only the active tab's reasoning effort", async () => {
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
            vision: true,
            curated: true,
            purposes: ['photo-analysis', 'writing'],
            reasoningEffort: { 'photo-analysis': 'high', writing: 'low' },
          },
        ],
      },
    })

    // The photo tab shows the photo effort.
    expect(await screen.findByRole('combobox', { name: /추론 강도 high/ })).toBeInTheDocument()

    // The writing tab shows its own, not the photo tab's.
    await user.click(screen.getByRole('tab', { name: '글 작성' }))
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: /추론 강도 low/ })).toBeInTheDocument(),
    )
    await chooseOption(user, screen.getByRole('combobox', { name: /추론 강도 low/ }), 'minimal')
    expect(calls).toContain('UpdateModel:writing:minimal')

    // Back on the photo tab, its effort is exactly where it was.
    await user.click(screen.getByRole('tab', { name: '사진 해석' }))
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: /추론 강도 high/ })).toBeInTheDocument(),
    )
  })

  // A11: an operator can see, per purpose, that a model is spending its budget on reasoning —
  // without reading the database and without waiting for somebody's job to fail.
  it('shows the recent reasoning spend beside the control it argues about', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          {
            modelId: 'anthropic/reasoner',
            label: 'Reasoner',
            vision: true,
            curated: true,
            purposes: ['photo-analysis', 'writing'],
            reasoningSpend: {
              'photo-analysis': { calls: 2n, reasoningTokens: 40n, completionTokens: 1_300n },
              writing: {
                calls: 3n,
                reasoningTokens: 24_000n,
                completionTokens: 24_300n,
                reasoningTruncations: 2n,
              },
            },
          },
        ],
      },
    })

    // The observation stage is fine, and says so without the warning.
    expect(
      await screen.findByText('완성 토큰의 3%가 추론에 쓰였어요 (2회 기준)'),
    ).toBeInTheDocument()
    expect(
      screen.queryByText(/설정한 강도를 이 모델이 따르지 않을 수 있어요/),
    ).not.toBeInTheDocument()

    // The writing stage is the 09-03 shape, and the tone is reinforced by words (§2.6).
    await user.click(screen.getByRole('tab', { name: '글 작성' }))
    expect(
      await screen.findByText('완성 토큰의 99%가 추론에 쓰였어요 (3회 기준)'),
    ).toBeInTheDocument()
    expect(screen.getByText(/설정한 강도를 이 모델이 따르지 않을 수 있어요/)).toBeInTheDocument()
    expect(screen.getByText('추론이 출력 예산을 소진한 호출 2회')).toBeInTheDocument()
  })

  // A model with no recorded call renders NOTHING: zero out of zero is the absence of a
  // measurement, and showing it as one is how an operator comes to trust a number nothing
  // stands behind.
  it('renders no spend signal for a model with no recorded calls', async () => {
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          {
            modelId: 'anthropic/fresh',
            label: 'Fresh',
            vision: true,
            curated: true,
            purposes: ['photo-analysis'],
          },
        ],
      },
    })

    await screen.findByRole('combobox', { name: /추론 강도/ })
    expect(screen.queryByText('최근 추론 사용')).not.toBeInTheDocument()
  })

  // The effort belongs to a REGISTRATION, so the control is absent on a tab the model does
  // not serve — and the server refuses one for such a purpose whatever a client renders.
  it('offers no reasoning control on a purpose the model is not registered to', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          {
            modelId: 'anthropic/claude-x',
            label: 'Claude X',
            vision: true,
            curated: true,
            purposes: ['photo-analysis'],
            reasoningEffort: { 'photo-analysis': 'high' },
          },
        ],
      },
    })

    expect(await screen.findByRole('combobox', { name: /추론 강도 high/ })).toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: '글 작성' }))
    await waitFor(() =>
      expect(screen.queryByRole('combobox', { name: /추론 강도/ })).not.toBeInTheDocument(),
    )
    // The cross-purpose registration is still visible without switching tabs — read off the
    // row itself, not off the tab strip that also carries the purpose's name.
    const row = screen.getByRole('listitem')
    expect(within(row).getByText(/사진 해석/)).toBeInTheDocument()
  })

  // A6 (plan 18): a curated model the provider no longer offers is badged and counted, and
  // nothing is deregistered on its behalf.
  it('flags a withdrawn model for the operator instead of retiring it', async () => {
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          {
            modelId: 'anthropic/claude-gone',
            label: 'Claude Gone',
            vision: true,
            curated: true,
            purposes: ['photo-analysis'],
            listed: false,
          },
        ],
      },
    })

    const row = await screen.findByRole('listitem')
    expect(within(row).getByText('제공 종료')).toBeInTheDocument()
    // Still registered: retiring it is the operator's decision, not the screen's.
    expect(within(row).getByRole('checkbox', { name: '이 용도에 사용' })).toBeChecked()
    expect(
      await screen.findByText(/모델 1개를 제공사가 더 이상 제공하지 않아요/),
    ).toBeInTheDocument()
  })

  // A7 (plan 18): an unreadable provider catalog degrades to curated rows and says so, rather
  // than looking like an empty catalog.
  it('degrades to curated rows when the provider catalog cannot be read', async () => {
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        fetchFails: true,
        entries: [
          {
            modelId: 'openai/gpt-new',
            label: 'GPT New',
            vision: true,
            curated: true,
            purposes: ['photo-analysis'],
          },
        ],
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
    // The refresh is scoped to the tab it was pressed on: the listing carries that purpose's
    // effort and that stage's spend signal.
    await waitFor(() => expect(calls).toContain('ListCatalog:refresh:photo-analysis'))
  })

  // --- the effort control is bounded by the source (change 27) ---

  // A3, against a real model: deepseek/deepseek-v4-pro-0813 accepts max·high·low and not
  // medium, and is disable-able yet lists no `none` — so the app's own "turn it off" value
  // must not be offered for it either.
  it("offers only the model's published efforts", async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          {
            modelId: 'deepseek/deepseek-v4-pro-0813',
            label: 'V4 Pro',
            curated: true,
            purposes: ['writing'],
            reasoning: { efforts: ['max', 'high', 'low'], defaultEffort: 'high' },
          },
        ],
      },
    })

    await user.click(await screen.findByRole('tab', { name: '글 작성' }))
    await user.click(await screen.findByRole('combobox', { name: /추론 강도/ }))
    const options = screen.getAllByRole('option').map((option) => option.textContent ?? '')
    // A5: `unset` is labelled with the model's own default, so it does not read as "off".
    expect(options).toEqual(['단계 기본값', 'unset (모델 기본값 high)', 'max', 'high', 'low'])
    for (const absent of ['medium', 'xhigh', 'minimal', 'none']) {
      expect(options).not.toContain(absent)
    }
  })

  // A4: a mandatory model offers no `none`, and a model with no published list still offers
  // all eight — absence is unknown, not "supports nothing".
  it('withholds none from a mandatory model and falls back for an unpublished list', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          {
            modelId: 'google/gemini-3.8-flash',
            label: 'Gemini Flash',
            curated: true,
            purposes: ['writing'],
            reasoning: { efforts: ['high', 'medium', 'low'], mandatory: true },
          },
          {
            modelId: 'vendor/no-list',
            label: 'No List',
            curated: true,
            purposes: ['writing'],
          },
        ],
      },
    })

    await user.click(await screen.findByRole('tab', { name: '글 작성' }))
    const rows = await screen.findAllByRole('listitem')
    const mandatory = rows.find((row) => within(row).queryByText(/gemini/) !== null)
    const listless = rows.find((row) => within(row).queryByText(/no-list/) !== null)
    if (!mandatory || !listless) throw new Error('a row is missing')

    await user.click(within(mandatory).getByRole('combobox', { name: /추론 강도/ }))
    let options = screen.getAllByRole('option').map((option) => option.textContent ?? '')
    expect(options).not.toContain('none')
    expect(options).toContain('medium')
    // `unset` has no default published, so it stays a bare label.
    expect(options).toContain('unset')
    await user.keyboard('{Escape}')

    await user.click(within(listless).getByRole('combobox', { name: /추론 강도/ }))
    options = screen.getAllByRole('option').map((option) => option.textContent ?? '')
    expect(options).toContain('none')
    expect(options).toContain('xhigh')
    expect(options).toHaveLength(9)
  })

  // A7: a drifted override survives, is still shown in the control, and is warned about —
  // never corrected. The same posture a delisted model gets.
  it('warns about a drifted override without changing it', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/models', {
      user: MASTER,
      calls,
      modelCatalog: {
        entries: [
          {
            modelId: 'deepseek/deepseek-v4-pro-0813',
            label: 'V4 Pro',
            curated: true,
            purposes: ['writing'],
            reasoningEffort: { writing: 'medium' },
            reasoning: { efforts: ['max', 'high', 'low'], defaultEffort: 'high' },
          },
        ],
      },
    })

    await user.click(await screen.findByRole('tab', { name: '글 작성' }))
    expect(await screen.findByRole('combobox', { name: /추론 강도 medium/ })).toBeInTheDocument()
    expect(
      await screen.findByText(/더 이상 「medium」을 지원 목록에 두지 않아요/),
    ).toBeInTheDocument()
    // Nothing was written: a warning is not a correction.
    expect(calls.filter((call) => call.startsWith('UpdateModel'))).toEqual([])
  })

  // A8: a model the source says does not reason has no effort to choose, so the control is
  // absent rather than offered and ignored. Its registration checkbox is untouched.
  it('shows no effort control for a model that does not reason', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        entries: [
          {
            modelId: 'vendor/no-reasoning',
            label: 'No Reasoning',
            curated: true,
            purposes: ['writing'],
            reasoning: { reasons: false },
          },
        ],
      },
    })

    await user.click(await screen.findByRole('tab', { name: '글 작성' }))
    const row = within((await screen.findAllByRole('listitem'))[0])
    expect(row.getByRole('checkbox', { name: '이 용도에 사용' })).toBeChecked()
    expect(row.queryByRole('combobox', { name: /추론 강도/ })).not.toBeInTheDocument()
  })

  // The failure mode the capability data introduces: when the provider catalog cannot be read,
  // every row is served from storage, whose `reasons` may predate this data entirely. Hiding
  // the control there would take away an override that is still sent on every call.
  it('keeps the effort control when the provider catalog cannot be read', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin/models', {
      user: MASTER,
      modelCatalog: {
        fetchFails: true,
        entries: [
          {
            modelId: 'vendor/never-refreshed',
            label: 'Never Refreshed',
            curated: true,
            purposes: ['writing'],
            reasoning: { reasons: false },
          },
        ],
      },
    })

    await user.click(await screen.findByRole('tab', { name: '글 작성' }))
    const control = await screen.findByRole('combobox', { name: /추론 강도/ })
    await user.click(control)
    // The full vocabulary, because nothing is known about this model's reasoning.
    expect(screen.getAllByRole('option')).toHaveLength(9)
  })

  // A1 (plan 17): the tab is master-only, redirected rather than refused — the account has a
  // session.
  it('sends a non-operator back to the app', async () => {
    renderAppAt('/admin/models', {
      user: { id: 'alice', plan: ProtoPlan.MAX },
      plans: { plan: ProtoPlan.MAX },
    })

    expect(await screen.findByRole('heading', { name: '내 글' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '모델 관리' })).not.toBeInTheDocument()
  })
})
