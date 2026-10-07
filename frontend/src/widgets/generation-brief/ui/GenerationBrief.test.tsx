import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react'
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
import { PostCreditsBasis, ProtoPlan, Stage } from '@/shared/api'
import type { FakeOptionsSave } from '@/test/posts'
import type { FakePlansOptions } from '@/test/plans'
import type { FakeProvidersOptions } from '@/test/providers'
import type { FakeQualityOptions } from '@/test/quality'
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
    createRoute({ getParentRoute: () => rootRoute, path: '/templates', component: () => null }),
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

/** The account's four readings, M2 over band so its row offers a tick. */
const QUALITY: FakeQualityOptions = {
  accounts: {
    'post-a': [
      {
        metric: 'title_saturation',
        verdict: 'below_minimum',
        minimum: 10,
        publishedCount: 5,
        values: { shareWarnAbove: 0.3 },
      },
      {
        metric: 'cross_post_phrases',
        verdict: 'over_band',
        minimum: 3,
        publishedCount: 5,
        ruleText: '다른 글에 있던 문장을 그대로 쓰지 마세요.',
        values: { share: 0.15, shareWarnAbove: 0.1 },
      },
      {
        metric: 'in_post_repetition',
        verdict: 'within_band',
        minimum: 1,
        publishedCount: 5,
        values: {
          repetitionShare: 0.05,
          titleRelevance: 0.7,
          repetitionShareWarnAbove: 0.08,
          titleRelevanceWarnBelow: 0.5,
        },
      },
      { metric: 'composition', verdict: 'absent', minimum: 3, publishedCount: 5 },
    ],
  },
}

type BriefOptions = NonNullable<Parameters<typeof GenerationBrief>[0]['options']>

function renderBrief(
  overrides: Partial<Parameters<typeof GenerationBrief>[0]> = {},
  {
    savedPair = false,
    quality,
    options,
    models,
    selections,
    plans,
  }: {
    savedPair?: boolean
    quality?: FakeQualityOptions
    options?: Partial<BriefOptions>
    models?: FakeProvidersOptions['models']
    selections?: FakeProvidersOptions['selections']
    plans?: FakePlansOptions
  } = {},
) {
  const calls: string[] = []
  const optionSaves: FakeOptionsSave[] = []
  const onSaved = vi.fn()
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    providers: {
      calls,
      // Three, not two: with each field hiding what the other holds, a two-model catalog cannot
      // show the difference between "excluded" and "the only one left".
      models: models ?? [
        { providerId: 'openrouter', modelId: 'writer', label: 'Writer', vision: true },
        { providerId: 'openrouter', modelId: 'rival', label: 'Rival', vision: true },
        { providerId: 'openrouter', modelId: 'third', label: 'Third', vision: true },
      ],
      selections: selections ?? [
        { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
      ],
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
    templates: { templates: [{ id: 'template-a', name: '일기' }] },
    posts: { posts: [{ slug: 'post-a' }], calls, optionSaves },
    quality,
    plans,
  })
  renderInRouter(
    <GenerationBrief
      targetLanguage="ko"
      onTargetLanguageSelect={vi.fn()}
      photoCount={0}
      options={{
        ownerId: 'alice',
        slug: 'post-a',
        saved: { tagCount: 4, useMemory: false, qualityRules: [], field: '' },
        jobRunning: false,
        onSaved,
        ...options,
      }}
      {...overrides}
    />,
    transport,
  )
  return { calls, optionSaves, onSaved }
}

const openBrief = async (user: ReturnType<typeof userEvent.setup>) => {
  await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
  return screen.getByRole('dialog', { name: '글쓰기 옵션' })
}

/** True when `a` comes before `b` in the document. */
const before = (a: Element, b: Element) =>
  Boolean(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING)

describe('GenerationBrief', () => {
  // A glyph-only trigger keeps its name: the surface it opens is what the button is called.
  it('names the surface on its closed icon trigger', async () => {
    renderBrief()
    expect(await screen.findByRole('button', { name: '글쓰기 옵션' })).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  // A1, narrowed: ONE surface holds every SETTING the run consumes, in the order it consumes them.
  // 말투 and 템플릿 are excepted — both are per-draft choices and both ride the dock's own row beside
  // this trigger, so getting either wrong is visible without opening anything.
  it('holds the run settings in one panel, without 말투 and 템플릿', async () => {
    const user = userEvent.setup()
    renderBrief()

    await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
    const panel = screen.getByRole('dialog', { name: '글쓰기 옵션' })

    for (const label of ['관찰 모델', '작성 모델', '글 언어']) {
      await waitFor(() =>
        expect(screen.getByRole('combobox', { name: new RegExp(label) })).toBeInTheDocument(),
      )
    }
    for (const absent of [/말투/, /템플릿/, /후보 A/, /후보 B/]) {
      expect(screen.queryByRole('combobox', { name: absent })).not.toBeInTheDocument()
    }
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeInTheDocument()
    // The tag count sits under the length, always visible, holding the post's value (POST-63).
    expect(screen.getByLabelText('태그 개수')).toHaveValue(4)
    // The legacy pair editor is absent; the common test flow receives named source context.
    expect(panel.textContent).not.toContain('AI 모델에서 두 후보 설정')
    expect(within(panel).queryByRole('link')).not.toBeInTheDocument()
  })

  it('opens the named common test only after newest material finishes saving', async () => {
    const user = userEvent.setup()
    let finishSaving!: () => void
    const saved = new Promise<void>((resolve) => {
      finishSaving = resolve
    })
    const navigate = vi.fn()
    const onOpen = vi.fn(async () => {
      await saved
      navigate()
    })
    const href = '/tests?sourcePost=post-a'
    const { calls } = renderBrief({ writingTest: { href, onOpen } })
    const panel = await openBrief(user)
    const link = within(panel).getByRole('link', { name: '글 설정 비교 테스트' })
    expect(link).toHaveAttribute('href', href)
    await user.click(link)
    expect(onOpen).toHaveBeenCalledTimes(1)
    expect(link).toHaveAttribute('aria-busy', 'true')
    expect(navigate).not.toHaveBeenCalled()
    expect(panel).toBeInTheDocument()
    expect(calls).not.toContain('SaveComparisonPair')
    await act(async () => {
      finishSaving()
      await saved
    })
    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('keeps the brief and source in place when the material flush fails', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn(async () => {
      throw new Error('private transport detail')
    })
    renderBrief({ writingTest: { href: '/tests?sourcePost=post-a', onOpen } })
    const panel = await openBrief(user)
    await user.click(within(panel).getByRole('link', { name: '글 설정 비교 테스트' }))
    const alert = await within(panel).findByRole('alert')
    expect(alert).toHaveTextContent('요청을 마치지 못했어요.')
    expect(alert).not.toHaveTextContent('private transport detail')
    expect(panel).toBeInTheDocument()
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  it('marks and focuses the ordinary active writer without test entrant fields', async () => {
    const user = userEvent.setup()
    renderBrief({ refusal: { mode: 'generation', count: 1 } }, { selections: [] })
    const panel = await openBrief(user)
    const writer = await within(panel).findByRole('combobox', { name: /^작성 모델/ })
    expect(writer).toHaveAttribute('aria-invalid', 'true')
    expect(writer).toHaveAccessibleDescription(
      expect.stringContaining('활성 작성 모델을 선택하세요.'),
    )
    await waitFor(() => expect(writer).toHaveFocus())
    expect(within(panel).queryByRole('combobox', { name: /후보/ })).not.toBeInTheDocument()
  })

  // POST-81: the 발행 글 점검 rows sit after 목표 분량, one per metric in catalogue order, each in the
  // state the server judged.
  it('renders the four quality states after 목표 분량', async () => {
    const user = userEvent.setup()
    renderBrief({}, { quality: QUALITY })

    await openBrief(user)
    const heading = await screen.findByText('발행 글 점검')
    expect(before(screen.getByLabelText('태그 개수'), heading)).toBe(true)
    expect(await screen.findByRole('checkbox', { name: /^글 간 고정 문구 15%/ })).toBeEnabled()
    expect(
      screen.getByText('발행한 글이 10편 이상이면 비교해요. 지금은 5편이에요.'),
    ).toBeInTheDocument()
    expect(screen.getByText('양호')).toBeInTheDocument()
    expect(screen.getByText('측정할 수 없어요')).toBeInTheDocument()
    const text = heading.parentElement?.textContent ?? ''
    const order = ['제목 도배율', '글 간 고정 문구', '글 안 반복과 제목 관련성', '분량·구성'].map(
      (name) => text.indexOf(name),
    )
    expect(order.every((at, i) => at >= 0 && (i === 0 || at > order[i - 1]))).toBe(true)
  })

  // POST-89: the five run options are ONE form, in the order the run reads them, closed by 저장.
  it('holds the run options in one form: 목표 분량, 발행 글 점검, 분야, 기억 사용, then 저장', async () => {
    const user = userEvent.setup()
    renderBrief({}, { quality: QUALITY })

    await openBrief(user)
    const length = screen.getByLabelText('목표 글자 수 사용')
    const quality = await screen.findByText('발행 글 점검')
    const field = screen.getByRole('group', { name: '분야' })
    const memory = screen.getByRole('checkbox', { name: '기억 사용' })
    const save = screen.getByRole('button', { name: '저장' })
    const order = [length, screen.getByLabelText('태그 개수'), quality, field, memory, save]
    expect(order.every((node, i) => i === 0 || before(order[i - 1], node))).toBe(true)
    const form = length.closest('form')
    expect(form).not.toBeNull()
    expect(order.every((node) => node.closest('form') === form)).toBe(true)
    // The model selects sit above it and save on their own.
    expect(before(screen.getByRole('combobox', { name: /작성 모델/ }), length)).toBe(true)
    expect(screen.getByRole('combobox', { name: /작성 모델/ }).closest('form')).toBeNull()
  })

  it('sends nothing while the options change, then all five in one request on 저장', async () => {
    const user = userEvent.setup()
    const { calls, optionSaves, onSaved } = renderBrief({}, { quality: QUALITY })

    await openBrief(user)
    await user.click(await screen.findByRole('checkbox', { name: /^글 간 고정 문구 15%/ }))
    await user.click(screen.getByRole('button', { name: '카페' }))
    await user.click(screen.getByRole('checkbox', { name: '기억 사용' }))
    await user.click(screen.getByLabelText('목표 글자 수 사용'))
    expect(calls).not.toContain('SavePostGenerationOptions')

    await user.click(screen.getByRole('button', { name: '저장' }))
    const sent = {
      targetLength: 1000,
      tagCount: 4,
      useMemory: true,
      qualityRules: ['cross_post_phrases'],
      field: 'cafe',
    }
    await waitFor(() => expect(optionSaves).toEqual([{ slug: 'post-a', ...sent }]))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(onSaved).toHaveBeenCalledWith(sent)
  })

  it('discards a change the brief closed without', async () => {
    const user = userEvent.setup()
    const { calls } = renderBrief()

    for (const close of ['취소', 'Escape']) {
      await openBrief(user)
      await user.click(screen.getByRole('button', { name: '카페' }))
      await user.click(screen.getByRole('checkbox', { name: '기억 사용' }))
      if (close === 'Escape') await user.keyboard('{Escape}')
      else await user.click(screen.getByRole('button', { name: close }))
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

      await openBrief(user)
      expect(screen.getByRole('button', { name: '없음' })).toHaveAttribute('aria-pressed', 'true')
      expect(screen.getByRole('button', { name: '카페' })).toHaveAttribute('aria-pressed', 'false')
      expect(screen.getByRole('checkbox', { name: '기억 사용' })).not.toBeChecked()
      await user.keyboard('{Escape}')
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    }
    expect(calls).not.toContain('SavePostGenerationOptions')
  })

  it('keeps 저장 off until something changes, and off again once it is undone', async () => {
    const user = userEvent.setup()
    renderBrief()

    await openBrief(user)
    const save = screen.getByRole('button', { name: '저장' })
    expect(save).toBeDisabled()
    await user.click(screen.getByRole('button', { name: '카페' }))
    expect(save).toBeEnabled()
    await user.click(screen.getByRole('button', { name: '없음' }))
    expect(save).toBeDisabled()
    await user.click(screen.getByRole('checkbox', { name: '기억 사용' }))
    expect(save).toBeEnabled()
    await user.click(screen.getByRole('checkbox', { name: '기억 사용' }))
    expect(save).toBeDisabled()
  })

  // A job reads the numbers and the ticks as it runs; 분야 and 기억 사용 are read at the next enqueue.
  it('holds the numbers and the ticks while a job runs, leaving 분야 and 기억 사용 usable', async () => {
    const user = userEvent.setup()
    renderBrief({}, { quality: QUALITY, options: { jobRunning: true } })

    await openBrief(user)
    expect(await screen.findByRole('checkbox', { name: /^글 간 고정 문구 15%/ })).toBeDisabled()
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeDisabled()
    expect(screen.getByLabelText('태그 개수')).toBeDisabled()
    expect(screen.getByRole('button', { name: '카페' })).toBeEnabled()
    expect(screen.getByRole('checkbox', { name: '기억 사용' })).toBeEnabled()
    await user.click(screen.getByRole('button', { name: '카페' }))
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
  })

  // POST-86: a published post's run options are shown and not changed; the account's model
  // settings stay usable.
  it('holds the whole form on a published post', async () => {
    const user = userEvent.setup()
    renderBrief({ locked: true }, { quality: QUALITY })

    await openBrief(user)
    expect(await screen.findByRole('checkbox', { name: /^글 간 고정 문구 15%/ })).toBeDisabled()
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeDisabled()
    expect(screen.getByLabelText('태그 개수')).toBeDisabled()
    const chips = within(screen.getByRole('group', { name: '분야' })).getAllByRole('button')
    expect(chips).toHaveLength(10)
    for (const chip of chips) expect(chip).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: '기억 사용' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    expect(screen.getByRole('combobox', { name: /관찰 모델/ })).toBeEnabled()
    expect(screen.getByRole('combobox', { name: /작성 모델/ })).toBeEnabled()
  })

  // POST-89: a draft with no post yet has no slug to save the run options against, so the whole
  // form is absent rather than offered and refused.
  it('omits the whole form before the post exists', async () => {
    const user = userEvent.setup()
    renderBrief({ options: undefined }, { quality: QUALITY })

    await openBrief(user)
    expect(await screen.findByRole('combobox', { name: /작성 모델/ })).toBeInTheDocument()
    expect(screen.queryByLabelText('목표 글자 수 사용')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('태그 개수')).not.toBeInTheDocument()
    expect(screen.queryByText('발행 글 점검')).not.toBeInTheDocument()
    expect(screen.queryByRole('group', { name: '분야' })).not.toBeInTheDocument()
    expect(screen.queryByRole('checkbox', { name: '기억 사용' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '저장' })).not.toBeInTheDocument()
  })
})

// QUOTA-64: the brief says what one post costs on the chosen pair in credits and how many the
// balance covers; the observe part counts only when this post has a photo.
describe('GenerationBrief per-post credits', () => {
  const models: FakeProvidersOptions['models'] = [
    {
      providerId: 'openrouter',
      modelId: 'eyes',
      label: 'Eyes',
      vision: true,
      postCredits: [{ stage: Stage.OBSERVE, credits: 30, basis: PostCreditsBasis.ESTIMATE }],
    },
    {
      providerId: 'openrouter',
      modelId: 'writer',
      label: 'Writer',
      postCredits: [{ stage: Stage.WRITE, credits: 12, basis: PostCreditsBasis.RECENT_USAGE }],
    },
  ]
  const selections: FakeProvidersOptions['selections'] = [
    { stage: Stage.OBSERVE, providerId: 'openrouter', modelId: 'eyes' },
    { stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' },
  ]
  const plans: FakePlansOptions = {
    plan: ProtoPlan.BASIC,
    balance: { credits: 100, unlimited: false },
  }

  it('prices a post with no photo on the write model alone', async () => {
    const user = userEvent.setup()
    renderBrief({ photoCount: 0 }, { models, selections, plans })
    const panel = await openBrief(user)
    expect(
      await within(panel).findByText('남은 크레딧으로 약 8편 쓸 수 있어요'),
    ).toBeInTheDocument()
    expect(within(panel).getByText(/최근 사용량 기준 글 1개당 약 12크레딧/)).toBeInTheDocument()
  })

  it('adds the observe part for a post with photos, and says the sum is an estimate', async () => {
    const user = userEvent.setup()
    renderBrief({ photoCount: 3 }, { models, selections, plans })
    const panel = await openBrief(user)
    expect(
      await within(panel).findByText('남은 크레딧으로 약 2편 쓸 수 있어요'),
    ).toBeInTheDocument()
    expect(within(panel).getByText(/예상 글 1개당 약 42크레딧/)).toBeInTheDocument()
    expect(within(panel).queryByText(/\$|USD|원가/)).not.toBeInTheDocument()
  })
})
