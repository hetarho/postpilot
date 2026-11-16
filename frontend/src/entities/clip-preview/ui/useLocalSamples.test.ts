import { renderHook } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createElement, type ReactNode } from 'react'
import { afterEach, expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import type { BrowserProjectCompositionInput } from '../model/project-composition'
import { useClipLocalCaptionPreview } from './useLocalSamples'

vi.mock('../model/browser-composition', () => ({
  freezeBrowserPreviewComposition: vi.fn(() => new Promise(() => {})),
}))

afterEach(() => vi.restoreAllMocks())

const wrapper = ({ children }: { children: ReactNode }) =>
  createElement(QueryClientProvider, { client: new QueryClient() }, children)

const sources: BrowserProjectCompositionInput['sources'] = []
type Plan = BrowserProjectCompositionInput['plan']
const inputFor = (
  plan: Plan,
  design: BrowserProjectCompositionInput['design'] = { captionPace: 'steady' },
): BrowserProjectCompositionInput => ({
  ownerId: 'owner',
  projectId: 'project',
  projectRevision: 1,
  planRevision: 1,
  plan,
  ratio: 'vertical',
  sources,
  design,
})

function planSerializations(spy: ReturnType<typeof vi.spyOn>, plan: Plan) {
  return spy.mock.calls.filter(([value]: unknown[]) => {
    const head = Array.isArray(value) ? (value[0] as { plan?: unknown } | undefined) : undefined
    return head?.plan === plan
  }).length
}

it('serializes the composition only when an input changes', () => {
  const plan = clipTimelineFixture().plan
  const spy = vi.spyOn(JSON, 'stringify')
  const { result, rerender } = renderHook(
    ({ input }) => useClipLocalCaptionPreview(input, true, 'caption'),
    { wrapper, initialProps: { input: inputFor(plan) } },
  )
  expect(result.current.isPending).toBe(true)
  const first = planSerializations(spy, plan)
  expect(first).toBeGreaterThan(0)

  rerender({ input: { ...inputFor(plan), projectRevision: 2, planRevision: 2 } })
  rerender({ input: inputFor(plan, { captionPace: 'steady' }) })
  expect(planSerializations(spy, plan)).toBe(first)

  const next = { ...plan }
  rerender({ input: inputFor(next) })
  expect(planSerializations(spy, next)).toBeGreaterThan(0)

  rerender({ input: inputFor(next, { captionPace: 'rapid' }) })
  expect(planSerializations(spy, next)).toBeGreaterThan(1)
})
