import { createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import { ClipGenerationService } from '@/shared/api'
import { withProviders } from '@/test/session'
import { clipProjectCalls } from './calls'
import { useClipAnalysisQuote } from './credits'

const models = {
  observeModel: { providerId: 'p', modelId: 'o' },
  writeModel: { providerId: 'p', modelId: 'w' },
}

// CLIP-234: 관찰하기 quotes and starts its own job, sending the sources named for 다시 관찰하기
// and reading back a quote that prices observations alone.
it('quotes and starts an analysis with the reanalysis list', async () => {
  const quoted: { reanalyzeSourceIds: string[] }[] = []
  const started: {
    reanalyzeSourceIds: string[]
    quoteId: string
    analysisPreparationId: string
  }[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipGenerationService.method.quoteClipAnalysis, (request) => {
      quoted.push(request)
      return {
        quoteId: 'quote',
        maxCredits: 4,
        expiresAt: '2099-01-01T00:00:00Z',
        reusedChunks: 1,
        remainingChunks: 2,
        pricedCalls: [{ label: 'observe', stage: 'observe', calls: 8 }],
      }
    })
    rpc(ClipGenerationService.method.startClipAnalysis, (request) => {
      started.push(request)
      return { jobId: 'job' }
    })
  })
  const view = renderHook(
    () =>
      useClipAnalysisQuote(
        'alice',
        { projectId: 'project', batchId: 'batch', ...models, reanalyzeSourceIds: ['source-a'] },
        'binding',
      ),
    { wrapper: withProviders(transport, new QueryClient()) },
  )
  await waitFor(() => expect(view.result.current.isSuccess).toBe(true))
  expect(quoted[0]?.reanalyzeSourceIds).toEqual(['source-a'])
  expect(view.result.current.data).toMatchObject({
    quoteId: 'quote',
    maxCredits: 4,
    binding: 'binding',
    calls: [{ label: 'observe', calls: 8 }],
  })
  const job = await clipProjectCalls(transport).startAnalysis({
    projectId: 'project',
    batchId: 'batch',
    ...models,
    quoteId: 'quote',
    approvedMaxCredits: 4,
    analysisPreparationId: 'prep',
    reanalyzeSourceIds: ['source-a'],
  })
  expect(job).toEqual({ jobId: 'job' })
  expect(started[0]).toMatchObject({
    quoteId: 'quote',
    analysisPreparationId: 'prep',
    reanalyzeSourceIds: ['source-a'],
  })
})

it('sends no reanalysis list for 관찰하기 and waits while disabled', async () => {
  const quoted: { reanalyzeSourceIds: string[] }[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipGenerationService.method.quoteClipAnalysis, (request) => {
      quoted.push(request)
      return { quoteId: 'quote', maxCredits: 0, expiresAt: '2099-01-01T00:00:00Z' }
    })
  })
  const input = { projectId: 'project', batchId: 'batch', ...models }
  const disabled = renderHook(() => useClipAnalysisQuote('alice', input, 'off', false), {
    wrapper: withProviders(transport, new QueryClient()),
  })
  expect(disabled.result.current.fetchStatus).toBe('idle')
  expect(quoted).toHaveLength(0)
  const view = renderHook(() => useClipAnalysisQuote('alice', input, 'on'), {
    wrapper: withProviders(transport, new QueryClient()),
  })
  await waitFor(() => expect(quoted).toHaveLength(1))
  expect(quoted[0]?.reanalyzeSourceIds).toEqual([])
  await waitFor(() => expect(view.result.current.fetchStatus).toBe('idle'))
})
