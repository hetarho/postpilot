import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClientProvider } from '@tanstack/react-query'
import {
  Outlet,
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { Stage } from '@/shared/api'
import { chooseOption } from '@/test/listbox'
import { createFakeAuthTransport, createTestQueryClient } from '@/test/session'
import { GenerationBrief } from './GenerationBrief'

afterEach(cleanup)

/** The brief composes features that may still route (the AI 모델 page owns the same pair), so it
 *  needs a router — but not the app's: a two-route memory tree keeps the test about the widget. */
function renderInRouter(
  ui: React.ReactNode,
  transport: ReturnType<typeof createFakeAuthTransport>,
) {
  const queryClient = createTestQueryClient()
  const rootRoute = createRootRoute({ component: Outlet })
  const routeTree = rootRoute.addChildren([
    createRoute({ getParentRoute: () => rootRoute, path: '/', component: () => ui }),
    createRoute({ getParentRoute: () => rootRoute, path: '/ai-models', component: () => null }),
    createRoute({ getParentRoute: () => rootRoute, path: '/purposes', component: () => null }),
  ])
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  return render(
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>,
  )
}

function renderBrief(
  overrides: Partial<Parameters<typeof GenerationBrief>[0]> = {},
  { savedPair = false }: { savedPair?: boolean } = {},
) {
  const calls: string[] = []
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    providers: {
      calls,
      // Three, not two: with each field hiding what the other holds, a two-model catalog cannot
      // show the difference between "excluded" and "the only one left".
      models: [
        { providerId: 'openrouter', modelId: 'writer', label: 'Writer', vision: true },
        { providerId: 'openrouter', modelId: 'rival', label: 'Rival', vision: true },
        { providerId: 'openrouter', modelId: 'third', label: 'Third', vision: true },
      ],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
      comparisonPairs: savedPair
        ? [
            {
              stage: Stage.WRITE,
              candidateA: { providerId: 'openrouter', modelId: 'writer' },
              candidateB: { providerId: 'openrouter', modelId: 'rival' },
            },
          ]
        : [],
    },
    voice: { voices: [{ id: 'voice-a', name: '일상 말투', isDefault: true }] },
    purposes: { purposes: [{ id: 'purpose-a', name: '일기' }] },
  })
  renderInRouter(
    <GenerationBrief
      targetLanguage="ko"
      onTargetLanguageSelect={vi.fn()}
      photoCount={0}
      targetLength={{
        slug: 'post-a',
        value: undefined,
        disabled: false,
        onSaved: vi.fn(),
      }}
      {...overrides}
    />,
    transport,
  )
  return { calls }
}

describe('GenerationBrief', () => {
  // A glyph-only trigger keeps its name: the surface it opens is what the button is called.
  it('names the surface on its closed icon trigger', async () => {
    renderBrief()
    expect(await screen.findByRole('button', { name: '글쓰기 옵션' })).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  // A1, narrowed: ONE surface holds every SETTING the run consumes, in the order it consumes them.
  // 말투 and 용도 are excepted — both are per-draft choices and both ride the dock's own row beside
  // this trigger, so getting either wrong is visible without opening anything.
  it('holds the run settings in one panel, without 말투 and 용도', async () => {
    const user = userEvent.setup()
    renderBrief()

    await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
    const panel = screen.getByRole('dialog', { name: '글쓰기 옵션' })

    for (const label of ['관찰 모델', '작성 모델', '후보 A', '후보 B', '글 언어']) {
      await waitFor(() =>
        expect(screen.getByRole('combobox', { name: new RegExp(label) })).toBeInTheDocument(),
      )
    }
    for (const absent of [/말투/, /용도/]) {
      expect(screen.queryByRole('combobox', { name: absent })).not.toBeInTheDocument()
    }
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeInTheDocument()
    // The A/B pair is chosen HERE now, so the way out to the AI 모델 page that stood in for it is
    // gone — following it mid-draft cost the user their place.
    expect(panel.textContent).not.toContain('AI 모델에서 두 후보 설정')
    expect(within(panel).queryByRole('link')).not.toBeInTheDocument()
  })

  // The pair has no 저장 button here, unlike the same fields on the AI 모델 page: the brief is a
  // surface of self-saving fields, so the save rides the second choice.
  it('saves the A/B pair as soon as both candidates name different models', async () => {
    const user = userEvent.setup()
    const { calls } = renderBrief()

    await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
    const candidateA = await screen.findByRole('combobox', { name: /후보 A/ })
    const candidateB = await screen.findByRole('combobox', { name: /후보 B/ })

    // One candidate is not a pair, and the backend has nothing to store for half of one.
    await chooseOption(user, candidateA, 'Writer')
    expect(calls).not.toContain('SaveComparisonPair')

    await chooseOption(user, candidateB, 'Rival')
    await waitFor(() => expect(calls).toContain('SaveComparisonPair'))
  })

  // The backend refuses two identical candidates, so neither field lists what the other holds and
  // the pair cannot be made duplicate in the first place.
  it('drops the model one candidate holds from the other candidate\u2019s list', async () => {
    const user = userEvent.setup()
    renderBrief()

    await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
    await chooseOption(user, await screen.findByRole('combobox', { name: /후보 A/ }), 'Writer')

    await user.click(await screen.findByRole('combobox', { name: /후보 B/ }))
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual([
      'Rival',
      'Third',
    ])
    expect(screen.queryByRole('option', { name: 'Writer' })).not.toBeInTheDocument()
  })

  // A blanked candidate is not a state the pair can be IN: `SaveComparisonPair` refuses an empty
  // ref and no RPC clears one. Offering the choice anyway emptied the field, sent nothing, and
  // left 글 생성's A/B 비교 running the candidate the user had just watched disappear.
  it('offers no way to blank a candidate the server could not clear', async () => {
    const user = userEvent.setup()
    renderBrief({}, { savedPair: true })

    await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
    const candidateA = await screen.findByRole('combobox', { name: /후보 A/ })
    await waitFor(() => expect(candidateA).toHaveTextContent('Writer'))

    // The panel lists models and nothing else: every row it offers is a choice this surface can
    // actually carry out.
    await user.click(candidateA)
    expect(screen.queryByRole('option', { name: '모델을 선택하세요' })).not.toBeInTheDocument()
    // Its own model, plus the one nobody holds. Rival is 후보 B's and is therefore not offered.
    expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual([
      'Writer',
      'Third',
    ])
  })

  // Never set is the field's EMPTY STATE rather than a listed choice, so it still reads as empty
  // before either candidate has been chosen.
  it('reads as unset while no pair has been saved', async () => {
    const user = userEvent.setup()
    renderBrief()

    await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
    const candidateA = await screen.findByRole('combobox', { name: /후보 A/ })
    expect(candidateA).toHaveTextContent('모델을 선택하세요')
  })

  // A1: a draft with no post yet has no slug to save a target length against, so that one field
  // is absent rather than offered and refused.
  it('omits 목표 분량 before the post exists', async () => {
    const user = userEvent.setup()
    renderBrief({ targetLength: undefined })

    await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
    expect(screen.queryByLabelText('목표 글자 수 사용')).not.toBeInTheDocument()
    expect(await screen.findByRole('combobox', { name: /작성 모델/ })).toBeInTheDocument()
  })
})
