import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { ProtoPlan, Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { chooseOption } from '@/test/listbox'
import type { FakeCatalogEntry } from '@/test/model-catalog'
import type {
  FakeModel,
  FakeProviderMutationFailure,
  FakeRecommendationSet,
} from '@/test/providers'

const MASTER = { id: 'root', plan: ProtoPlan.MASTER }
const ref = (modelId: string) => ({ providerId: 'openrouter', modelId })

/** What the operator may put in a slot: registered AND classified at the stage (MODEL-70). */
const MODELS: FakeModel[] = [
  {
    providerId: 'openrouter',
    modelId: 'vendor/eyes',
    label: 'Eyes',
    vision: true,
    stages: [Stage.OBSERVE, Stage.WRITE, Stage.ANALYZE],
    levels: { [Stage.OBSERVE]: 'value', [Stage.WRITE]: 'value', [Stage.ANALYZE]: 'value' },
  },
  {
    providerId: 'openrouter',
    modelId: 'vendor/sight',
    label: 'Sight',
    vision: true,
    stages: [Stage.OBSERVE, Stage.WRITE],
    levels: { [Stage.OBSERVE]: 'top', [Stage.WRITE]: 'top' },
  },
]

/** The operator's catalog reads, which the read-time flags come from. `vendor/draft` is
 *  registered to writing without a grade; `vendor/gone` is not in the catalog at all. */
const CATALOG: FakeCatalogEntry[] = [
  {
    modelId: 'vendor/eyes',
    label: 'Eyes',
    vision: true,
    curated: true,
    purposes: ['photo-analysis', 'style-analysis', 'writing'],
    level: { 'photo-analysis': 'value', 'style-analysis': 'value', writing: 'value' },
  },
  {
    modelId: 'vendor/sight',
    label: 'Sight',
    vision: true,
    curated: true,
    purposes: ['photo-analysis', 'writing'],
    level: { 'photo-analysis': 'top', writing: 'top' },
  },
  { modelId: 'vendor/draft', label: 'Draft', curated: true, purposes: ['writing'] },
]

function set(id: string, label: string, writeB = 'vendor/sight'): FakeRecommendationSet {
  return {
    id,
    label,
    selections: [
      {
        stage: Stage.OBSERVE,
        active: ref('vendor/eyes'),
        candidateA: ref('vendor/eyes'),
        candidateB: ref('vendor/sight'),
      },
      { stage: Stage.ANALYZE, active: ref('vendor/eyes') },
      {
        stage: Stage.WRITE,
        active: ref('vendor/eyes'),
        candidateA: ref('vendor/eyes'),
        candidateB: ref(writeB),
      },
    ],
  }
}

/** Opens 모델 관리 and its sixth tab, where the sets live (MODEL-28, MODEL-69). */
async function renderTab(options: {
  sets: FakeRecommendationSet[]
  calls?: string[]
  saveRecommendationFailure?: FakeProviderMutationFailure
}) {
  const view = renderAppAt('/admin/models', {
    user: MASTER,
    calls: options.calls,
    providers: {
      calls: options.calls,
      models: MODELS,
      recommendationSets: options.sets,
      saveRecommendationFailure: options.saveRecommendationFailure,
    },
    modelCatalog: { entries: CATALOG },
  })
  await userEvent.setup().click(await screen.findByRole('tab', { name: '추천 조합' }))
  return view
}

async function findTab() {
  return within(await screen.findByRole('region', { name: '추천 조합' }))
}

describe('the 추천 조합 tab', () => {
  // MODEL-28: /admin keeps four tabs, and 추천 조합 is 모델 관리's sixth, after the five purposes.
  // It is not a purpose: the search, filters and provider refresh go away, 일괄 편집 stays.
  it('is 모델 관리’s sixth tab and keeps only the shared 일괄 편집 entry', async () => {
    await renderTab({ sets: [] })
    await findTab()
    const adminTabs = within(screen.getByRole('navigation', { name: '운영 관리' }))
      .getAllByRole('link')
      .map((link) => link.getAttribute('href'))
    expect(adminTabs).toEqual([
      '/admin',
      '/admin/models',
      '/admin/estimator',
      '/admin/vouchers',
      '/admin/costs',
    ])
    const tabs = within(screen.getByRole('tablist', { name: '모델 용도' })).getAllByRole('tab')
    expect(tabs.map((tab) => tab.textContent)).toEqual([
      '사진 해석',
      '문체 분석',
      '글 작성',
      '이미지 생성',
      '비디오 생성',
      '추천 조합',
      '목소리',
    ])
    expect(tabs[5]).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('button', { name: '일괄 편집' })).toBeInTheDocument()
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '목록 새로고침' })).not.toBeInTheDocument()

    await userEvent.setup().click(tabs[2])
    expect(await screen.findByRole('searchbox')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '목록 새로고침' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '추천 조합' })).not.toBeInTheDocument()
  })

  // MODEL-70: a saved set keeps a model the catalog has since moved away from, and the list
  // says so beside the slot rather than editing the set.
  it('lists each set with its grades and flags slots the catalog no longer supports', async () => {
    await renderTab({
      sets: [
        {
          ...set('balanced', 'Balanced', 'vendor/draft'),
          selections: set('balanced', 'Balanced', 'vendor/draft').selections.map((selection) =>
            selection.stage === Stage.OBSERVE
              ? { ...selection, candidateB: ref('vendor/gone') }
              : selection,
          ),
        },
      ],
    })
    const tab = await findTab()
    const card = within(await tab.findByRole('group', { name: 'Balanced' }))
    expect(card.getByText('사진 관찰')).toBeInTheDocument()
    expect(card.getByText('문체 분석')).toBeInTheDocument()
    expect(card.getByText('글 작성')).toBeInTheDocument()
    expect((await card.findAllByText('가성비')).length).toBeGreaterThan(0)
    expect(await card.findByText('등록 해제됨')).toBeInTheDocument()
    expect(card.getByText('미분류')).toBeInTheDocument()
    expect(card.getByText('vendor/gone')).toBeInTheDocument()
  })

  it('creates a set from seven chosen slots and lists it last', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    await renderTab({ sets: [set('first', 'First')], calls })
    const tab = await findTab()
    await tab.findByRole('group', { name: 'First' })

    await user.click(tab.getByRole('button', { name: '조합 추가' }))
    const editor = within(await tab.findByRole('form', { name: '새 추천 조합' }))
    await user.type(editor.getByRole('textbox', { name: '이름' }), '  Second  ')
    for (const [name, option] of [
      [/사진 관찰 활성/, /Eyes/],
      [/사진 관찰 A/, /Eyes/],
      [/사진 관찰 B/, /Sight/],
      [/문체 분석 활성/, /Eyes/],
      [/글 작성 활성/, /Sight/],
      [/글 작성 A/, /Sight/],
      [/글 작성 B/, /Eyes/],
    ] as const) {
      await chooseOption(user, editor.getByRole('combobox', { name }), option)
    }
    await user.click(editor.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(calls).toContain('SaveRecommendationSet:new:  Second  '))
    // The server stores the label trimmed, and the editor closes onto the refreshed list.
    await tab.findByRole('group', { name: 'Second' })
    expect(tab.getAllByRole('heading', { level: 4 }).map((heading) => heading.textContent)).toEqual(
      ['First', 'Second'],
    )
  })

  // MODEL-70: the server names every offending field at once; each cause sits beside its own
  // field and the draft survives the refusal.
  it('shows every refused field beside itself and keeps the draft', async () => {
    const user = userEvent.setup()
    await renderTab({
      sets: [],
      saveRecommendationFailure: {
        reason: 'MODEL_SET_INVALID',
        params: {
          fields: 'label,observe_candidate_b,write_active',
          label: 'required',
          observe_candidate_b: 'duplicate',
          write_active: 'unclassified',
        },
      },
    })
    const tab = await findTab()
    await user.click(await tab.findByRole('button', { name: '조합 추가' }))
    const editor = within(await tab.findByRole('form', { name: '새 추천 조합' }))
    await chooseOption(user, editor.getByRole('combobox', { name: /사진 관찰 A/ }), /Eyes/)
    await user.click(editor.getByRole('button', { name: '저장' }))

    expect(
      await editor.findByText('추천 조합을 저장하지 못했어요. 표시된 칸을 고쳐 주세요.'),
    ).toBeInTheDocument()
    expect(editor.getByText('값을 채워 주세요.')).toBeInTheDocument()
    expect(editor.getByText('A와 다른 모델을 고르세요.')).toBeInTheDocument()
    expect(editor.getByText('운영자가 아직 등급을 분류하지 않은 모델이에요.')).toBeInTheDocument()
    expect(editor.getByRole('combobox', { name: /사진 관찰 B/ })).toHaveAttribute(
      'aria-invalid',
      'true',
    )
    expect(editor.getByRole('combobox', { name: /사진 관찰 A/ })).toHaveAccessibleName(/Eyes/)
  })

  // MODEL-69: labels are unique; a clash is named as a name problem, not a repeated model.
  it('says a label is already taken', async () => {
    const user = userEvent.setup()
    await renderTab({
      sets: [],
      saveRecommendationFailure: {
        reason: 'MODEL_SET_INVALID',
        params: { fields: 'label', label: 'duplicate' },
      },
    })
    const tab = await findTab()
    await user.click(await tab.findByRole('button', { name: '조합 추가' }))
    const editor = within(await tab.findByRole('form', { name: '새 추천 조합' }))
    await user.type(editor.getByRole('textbox', { name: '이름' }), 'Balanced')
    await user.click(editor.getByRole('button', { name: '저장' }))
    expect(await editor.findByText('다른 조합이 이미 쓰는 이름이에요.')).toBeInTheDocument()
    expect(editor.queryByText('A와 다른 모델을 고르세요.')).not.toBeInTheDocument()
  })

  it('edits a set from its saved values', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    await renderTab({ sets: [set('first', 'First')], calls })
    const tab = await findTab()
    await user.click(await tab.findByRole('button', { name: 'First 수정' }))
    const editor = within(await tab.findByRole('form', { name: '추천 조합 수정' }))
    const name = editor.getByRole('textbox', { name: '이름' })
    expect(name).toHaveValue('First')
    expect(editor.getByRole('combobox', { name: /사진 관찰 B/ })).toHaveAccessibleName(/Sight/)
    await user.clear(name)
    await user.type(name, 'Renamed')
    await user.click(editor.getByRole('button', { name: '저장' }))
    await waitFor(() => expect(calls).toContain('SaveRecommendationSet:first:Renamed'))
    expect(await tab.findByRole('group', { name: 'Renamed' })).toBeInTheDocument()
  })

  it('moves sets one place and disables the move at either end', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    await renderTab({ sets: [set('first', 'First'), set('second', 'Second')], calls })
    const tab = await findTab()
    await tab.findByRole('group', { name: 'Second' })
    expect(tab.getByRole('button', { name: 'First 위로 옮기기' })).toBeDisabled()
    expect(tab.getByRole('button', { name: 'Second 아래로 옮기기' })).toBeDisabled()

    await user.click(tab.getByRole('button', { name: 'Second 위로 옮기기' }))
    await waitFor(() => expect(calls).toContain('MoveRecommendationSet:second:up'))
    await waitFor(() =>
      expect(tab.getAllByRole('heading', { level: 4 }).map((h) => h.textContent)).toEqual([
        'Second',
        'First',
      ]),
    )
  })

  it('deletes a set only after the confirmation', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    await renderTab({ sets: [set('first', 'First')], calls })
    const tab = await findTab()
    await user.click(await tab.findByRole('button', { name: 'First 삭제' }))
    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText(/"First" 조합이 모델 변경 화면에서 사라집니다/)).toBeInTheDocument()
    expect(calls.filter((call) => call.startsWith('DeleteRecommendationSet'))).toEqual([])

    await user.click(dialog.getByRole('button', { name: '삭제' }))
    await waitFor(() => expect(calls).toContain('DeleteRecommendationSet:first'))
    expect(await tab.findByText(/아직 추천 조합이 없어요/)).toBeInTheDocument()
  })

  // MODEL-69: at most ten sets.
  it('stops offering a new set at the limit', async () => {
    await renderTab({
      sets: Array.from({ length: 10 }, (_, index) => set(`set-${index}`, `Set ${index}`)),
    })
    const tab = await findTab()
    await tab.findByRole('group', { name: 'Set 9' })
    expect(tab.getByRole('button', { name: '조합 추가' })).toBeDisabled()
    expect(
      tab.getByText('추천 조합은 10개까지 만들 수 있어요. 하나를 지운 뒤 추가하세요.'),
    ).toBeInTheDocument()
  })
})
