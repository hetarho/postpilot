import { cleanup, render, screen, waitFor } from '@testing-library/react'
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
import { createFakeAuthTransport, createTestQueryClient } from '@/test/session'
import { GenerationBrief } from './GenerationBrief'

afterEach(cleanup)

/** The brief holds real router links (`/ai-models`, `/purposes`), so it needs a router — but not
 *  the app's: a two-route memory tree keeps the test about the widget. */
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

function renderBrief(overrides: Partial<Parameters<typeof GenerationBrief>[0]> = {}) {
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    providers: {
      models: [{ providerId: 'openrouter', modelId: 'writer', label: 'Writer', vision: true }],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
    },
    voice: { voices: [{ id: 'voice-a', name: '일상 말투', isDefault: true }] },
    purposes: { purposes: [{ id: 'purpose-a', name: '일기' }] },
  })
  renderInRouter(
    <GenerationBrief
      ownerId="alice"
      purposeId=""
      onPurposeSelect={vi.fn()}
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
}

describe('GenerationBrief', () => {
  // A glyph-only trigger keeps its name: the surface it opens is what the button is called.
  it('names the surface on its closed icon trigger', async () => {
    renderBrief()
    expect(await screen.findByRole('button', { name: '글쓰기 옵션' })).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  // A1: ONE surface holds the whole brief, in the order the run consumes it — 말투 excepted, which
  // rides the dock's own row beside this trigger so a wrong voice is visible without opening it.
  it('holds the whole writing brief in one panel, without 말투', async () => {
    const user = userEvent.setup()
    renderBrief()

    await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
    const panel = screen.getByRole('dialog', { name: '글쓰기 옵션' })

    for (const label of ['관찰 모델', '작성 모델', '용도', '글 언어']) {
      await waitFor(() =>
        expect(screen.getByRole('combobox', { name: new RegExp(label) })).toBeInTheDocument(),
      )
    }
    expect(screen.queryByRole('combobox', { name: /말투/ })).not.toBeInTheDocument()
    expect(screen.getByLabelText('목표 글자 수 사용')).toBeInTheDocument()
    expect(panel.textContent).toContain('AI 모델에서 두 후보 설정')
  })

  // A1: a draft with no post yet has no slug to save a target length against, so that one field
  // is absent rather than offered and refused.
  it('omits 목표 분량 before the post exists', async () => {
    const user = userEvent.setup()
    renderBrief({ targetLength: undefined })

    await user.click(await screen.findByRole('button', { name: '글쓰기 옵션' }))
    expect(screen.queryByLabelText('목표 글자 수 사용')).not.toBeInTheDocument()
    expect(await screen.findByRole('combobox', { name: /용도/ })).toBeInTheDocument()
  })
})
